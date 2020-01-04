package main

import (
	"crypto/subtle"
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"path"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/guggero/chantools/btc"
	"github.com/guggero/chantools/dataformat"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/keychain"
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
	RootKey   string `long:"rootkey" description:"BIP32 HD root key to use."`
	ChannelDB string `long:"channeldb" description:"The lnd channel.db file to use for rescuing force-closed channels."`
}

func (c *rescueClosedCommand) Execute(_ []string) error {
	// Check that root key is valid.
	if c.RootKey == "" {
		return fmt.Errorf("root key is required")
	}
	extendedKey, err := hdkeychain.NewKeyFromString(c.RootKey)
	if err != nil {
		return fmt.Errorf("error parsing root key: %v", err)
	}

	// Check that we have a channel DB.
	if c.ChannelDB == "" {
		return fmt.Errorf("rescue DB is required")
	}
	db, err := channeldb.Open(path.Dir(c.ChannelDB))
	if err != nil {
		return fmt.Errorf("error opening rescue DB: %v", err)
	}

	// Parse channel entries from any of the possible input files.
	entries, err := parseInput(cfg)
	if err != nil {
		return err
	}
	return rescueClosedChannels(extendedKey, entries, db)
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
	fileName := fmt.Sprintf("results/bruteforce-%s.json",
		time.Now().Format("2006-01-02-15-04-05"))
	log.Infof("Writing result to %s", fileName)
	return ioutil.WriteFile(fileName, summaryBytes, 0644)
}

func addrInCache(addr string, perCommitPoint *btcec.PublicKey) (string, error) {
	targetPubKeyHash, err := parseAddr(addr)
	if err != nil {
		return "", fmt.Errorf("error parsing addr: %v", err)
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
			targetPubKeyHash[:], hashedPubKey[:],
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
		key, err := btc.DeriveChildren(extendedKey, []uint32{
			btc.HardenedKeyStart + uint32(keychain.BIP0043Purpose),
			btc.HardenedKeyStart + chainParams.HDCoinType,
			btc.HardenedKeyStart +
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

func parseAddr(addr string) ([]byte, error) {
	// First parse address to get targetPubKeyHash from it later.
	targetAddr, err := btcutil.DecodeAddress(addr, chainParams)
	if err != nil {
		return nil, err
	}

	var targetPubKeyHash []byte
	// Make the check on the decoded address according to the active
	// network (testnet or mainnet only).
	if !targetAddr.IsForNet(chainParams) {
		return nil, fmt.Errorf(
			"address: %v is not valid for this network: %v",
			targetAddr.String(), chainParams.Name,
		)
	}

	// Must be a bech32 native SegWit address.
	switch targetAddr.(type) {
	case *btcutil.AddressWitnessPubKeyHash:
		targetPubKeyHash = targetAddr.ScriptAddress()
	default:
		return nil, fmt.Errorf("address: must be a bech32 P2WPKH address")
	}
	return targetPubKeyHash, nil
}
