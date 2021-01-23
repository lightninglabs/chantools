package main

import (
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/guggero/chantools/dataformat"
	"github.com/guggero/chantools/lnd"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/spf13/cobra"
)

var (
	cacheSize = 2000
	cache     []*cacheEntry

	errAddrNotFound = errors.New("addr not found")
)

type cacheEntry struct {
	privKey *btcec.PrivateKey
	pubKey  *btcec.PublicKey
}

type rescueClosedCommand struct {
	ChannelDB   string
	Addr        string
	CommitPoint string

	rootKey *rootKey
	inputs  *inputFlags
	cmd     *cobra.Command
}

func newRescueClosedCommand() *cobra.Command {
	cc := &rescueClosedCommand{}
	cc.cmd = &cobra.Command{
		Use: "rescueclosed",
		Short: "Try finding the private keys for funds that " +
			"are in outputs of remotely force-closed channels",
		Long: `If channels have already been force-closed by the remote
peer, this command tries to find the private keys to sweep the funds from the
output that belongs to our side. This can only be used if we have a channel DB
that contains the latest commit point. Normally you would use SCB to get the
funds from those channels. But this method can help if the other node doesn't
know about the channels any more but we still have the channel.db from the
moment they force-closed.

The alternative use case for this command is if you got the commit point by
running the fund-recovery branch of my guggero/lnd fork in combination with the
fakechanbackup command. Then you need to specify the --commit_point and 
--force_close_addr flags instead of the --channeldb and --fromsummary flags.`,
		Example: `chantools rescueclosed --rootkey xprvxxxxxxxxxx \
	--fromsummary results/summary-xxxxxx.json \
	--channeldb ~/.lnd/data/graph/mainnet/channel.db

chantools rescueclosed --rootkey xprvxxxxxxxxxx \
	--force_close_addr bc1q... \
	--commit_point 03xxxx`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.ChannelDB, "channeldb", "", "lnd channel.db file to use "+
			"for rescuing force-closed channels",
	)
	cc.cmd.Flags().StringVar(
		&cc.Addr, "force_close_addr", "", "the address the channel "+
			"was force closed to",
	)
	cc.cmd.Flags().StringVar(
		&cc.CommitPoint, "commit_point", "", "the commit point that "+
			"was obtained from the logs after running the "+
			"fund-recovery branch of guggero/lnd",
	)

	cc.rootKey = newRootKey(cc.cmd, "decrypting the backup")
	cc.inputs = newInputFlags(cc.cmd)

	return cc.cmd
}

func (c *rescueClosedCommand) Execute(_ *cobra.Command, _ []string) error {
	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %v", err)
	}

	// What way of recovery has the user chosen? From summary and DB or from
	// address and commit point?
	switch {
	case c.ChannelDB != "":
		db, err := lnd.OpenDB(c.ChannelDB, true)
		if err != nil {
			return fmt.Errorf("error opening rescue DB: %v", err)
		}

		// Parse channel entries from any of the possible input files.
		entries, err := c.inputs.parseInputType()
		if err != nil {
			return err
		}
		return rescueClosedChannels(extendedKey, entries, db)

	case c.Addr != "":
		// First parse address to get targetPubKeyHash from it later.
		targetAddr, err := btcutil.DecodeAddress(c.Addr, chainParams)
		if err != nil {
			return fmt.Errorf("error parsing addr: %v", err)
		}

		// Now parse the commit point.
		commitPointRaw, err := hex.DecodeString(c.CommitPoint)
		if err != nil {
			return fmt.Errorf("error decoding commit point: %v",
				err)
		}
		commitPoint, err := btcec.ParsePubKey(
			commitPointRaw, btcec.S256(),
		)
		if err != nil {
			return fmt.Errorf("error parsing commit point: %v", err)
		}

		return rescueClosedChannel(extendedKey, targetAddr, commitPoint)

	default:
		return fmt.Errorf("you either need to specify --channeldb and " +
			"--fromsummary or --force_close_addr and " +
			"--commit_point but not a mixture of them")
	}
}

func rescueClosedChannels(extendedKey *hdkeychain.ExtendedKey,
	entries []*dataformat.SummaryEntry, chanDb *channeldb.DB) error {

	err := fillCache(extendedKey)
	if err != nil {
		return err
	}

	channels, err := chanDb.FetchAllChannels()
	if err != nil {
		return err
	}

	// Try naive/lucky guess with information from channel DB.
	for _, channel := range channels {
		channelPoint := channel.FundingOutpoint.String()
		var channelEntry *dataformat.SummaryEntry
		for _, entry := range entries {
			if entry.ChannelPoint == channelPoint {
				channelEntry = entry
			}
		}

		// Don't try anything with open channels, fully closed channels
		// or channels where we already have the private key.
		if channelEntry == nil || channelEntry.ClosingTX == nil ||
			channelEntry.ClosingTX.AllOutsSpent ||
			channelEntry.ClosingTX.OurAddr == "" ||
			channelEntry.ClosingTX.SweepPrivkey != "" {
			continue
		}

		if channel.RemoteNextRevocation != nil {
			wif, err := addrInCache(
				channelEntry.ClosingTX.OurAddr,
				channel.RemoteNextRevocation,
			)
			switch {
			case err == nil:
				channelEntry.ClosingTX.SweepPrivkey = wif

			case err == errAddrNotFound:

			default:
				return err
			}
		}

		if channel.RemoteCurrentRevocation != nil {
			wif, err := addrInCache(
				channelEntry.ClosingTX.OurAddr,
				channel.RemoteCurrentRevocation,
			)
			switch {
			case err == nil:
				channelEntry.ClosingTX.SweepPrivkey = wif

			case err == errAddrNotFound:

			default:
				return err
			}
		}
	}

	summaryBytes, err := json.MarshalIndent(&dataformat.SummaryEntryFile{
		Channels: entries,
	}, "", " ")
	if err != nil {
		return err
	}
	fileName := fmt.Sprintf("results/rescueclosed-%s.json",
		time.Now().Format("2006-01-02-15-04-05"))
	log.Infof("Writing result to %s", fileName)
	return ioutil.WriteFile(fileName, summaryBytes, 0644)
}

func rescueClosedChannel(extendedKey *hdkeychain.ExtendedKey,
	addr btcutil.Address, commitPoint *btcec.PublicKey) error {

	// Make the check on the decoded address according to the active
	// network (testnet or mainnet only).
	if !addr.IsForNet(chainParams) {
		return fmt.Errorf("address: %v is not valid for this network: "+
			"%v", addr, chainParams.Name)
	}

	// Must be a bech32 native SegWit address.
	switch addr.(type) {
	case *btcutil.AddressWitnessPubKeyHash:
		log.Infof("Brute forcing private key for tweaked public key "+
			"hash %x\n", addr.ScriptAddress())

	default:
		return fmt.Errorf("address: must be a bech32 P2WPKH address")
	}

	err := fillCache(extendedKey)
	if err != nil {
		return err
	}

	wif, err := addrInCache(addr.String(), commitPoint)
	switch {
	case err == nil:
		log.Infof("Found private key %s for address %v!", wif, addr)

		return nil

	case err == errAddrNotFound:
		return fmt.Errorf("did not find private key for address %v",
			addr)

	default:
		return err
	}
}

func addrInCache(addr string, perCommitPoint *btcec.PublicKey) (string, error) {
	targetPubKeyHash, scriptHash, err := lnd.DecodeAddressHash(
		addr, chainParams,
	)
	if err != nil {
		return "", fmt.Errorf("error parsing addr: %v", err)
	}
	if scriptHash {
		return "", fmt.Errorf("address must be a P2WPKH address")
	}

	// Loop through all cached payment base point keys, tweak each of it
	// with the per_commit_point and see if the hashed public key
	// corresponds to the target pubKeyHash of the given address.
	for i := 0; i < cacheSize; i++ {
		cacheEntry := cache[i]
		basePoint := cacheEntry.pubKey
		tweakedPubKey := input.TweakPubKey(basePoint, perCommitPoint)
		tweakBytes := input.SingleTweakBytes(perCommitPoint, basePoint)
		tweakedPrivKey := input.TweakPrivKey(
			cacheEntry.privKey, tweakBytes,
		)
		hashedPubKey := btcutil.Hash160(
			tweakedPubKey.SerializeCompressed(),
		)
		equal := subtle.ConstantTimeCompare(
			targetPubKeyHash, hashedPubKey,
		)
		if equal == 1 {
			wif, err := btcutil.NewWIF(
				tweakedPrivKey, chainParams, true,
			)
			if err != nil {
				return "", err
			}
			log.Infof("The private key for addr %s found after "+
				"%d tries: %s", addr, i, wif.String(),
			)
			return wif.String(), nil
		}
	}

	return "", errAddrNotFound
}

func fillCache(extendedKey *hdkeychain.ExtendedKey) error {
	cache = make([]*cacheEntry, cacheSize)

	for i := 0; i < cacheSize; i++ {
		key, err := lnd.DeriveChildren(extendedKey, []uint32{
			lnd.HardenedKeyStart + uint32(keychain.BIP0043Purpose),
			lnd.HardenedKeyStart + chainParams.HDCoinType,
			lnd.HardenedKeyStart +
				uint32(keychain.KeyFamilyPaymentBase),
			0,
			uint32(i),
		})
		if err != nil {
			return err
		}
		privKey, err := key.ECPrivKey()
		if err != nil {
			return err
		}
		pubKey, err := key.ECPubKey()
		if err != nil {
			return err
		}
		cache[i] = &cacheEntry{
			privKey: privKey,
			pubKey:  pubKey,
		}

		if i > 0 && i%10000 == 0 {
			fmt.Printf("Filled cache with %d of %d keys.\n",
				i, cacheSize)
		}

	}
	return nil
}
