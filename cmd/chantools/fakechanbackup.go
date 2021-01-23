package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcutil"
	"github.com/guggero/chantools/lnd"
	"github.com/lightningnetwork/lnd/chanbackup"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/spf13/cobra"
)

type fakeChanBackupCommand struct {
	NodeAddr     string
	ChannelPoint string
	ShortChanID  string
	Initiator    bool
	Capacity     uint64
	MultiFile    string

	rootKey *rootKey
	cmd     *cobra.Command
}

func newFakeChanBackupCommand() *cobra.Command {
	cc := &fakeChanBackupCommand{}
	cc.cmd = &cobra.Command{
		Use:   "fakechanbackup",
		Short: "Fake a channel backup file to attempt fund recovery",
		Long: `If for any reason a node suffers from data loss and there is no
channel.backup for one or more channels, then the funds in the channel would
theoretically be lost forever.
If the remote node is still online and still knows about the channel, there is
hope. We can initiate DLP (Data Loss Protocol) and ask the remote node to
force-close the channel and to provide us with the per_commit_point that is
needed to derive the private key for our part of the force-close transaction
output. But to initiate DLP, we would need to have a channel.backup file.
Fortunately, if we have enough information about the channel, we can create a
faked/skeleton channel.backup file that at least lets us talk to the other node
and ask them to do their part. Then we can later brute-force the private key for
the transaction output of our part of the funds (see rescueclosed command).`,
		Example: `chantools fakechanbackup --rootkey xprvxxxxxxxxxx \
	--capacity 123456 \
	--channelpoint f39310xxxxxxxxxx:1 \
	--initiator \
	--remote_node_addr 022c260xxxxxxxx@213.174.150.1:9735 \
	--short_channel_id 566222x300x1 \
	--multi_file fake.backup`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.NodeAddr, "remote_node_addr", "", "the remote node "+
			"connection information in the format pubkey@host:"+
			"port",
	)
	cc.cmd.Flags().StringVar(
		&cc.ChannelPoint, "channelpoint", "", "funding transaction "+
			"outpoint of the channel to rescue (<txid>:<txindex>) "+
			"as it is displayed on 1ml.com",
	)
	cc.cmd.Flags().StringVar(
		&cc.ShortChanID, "short_channel_id", "", "the short channel "+
			"ID in the format <blockheight>x<transactionindex>x"+
			"<outputindex>",
	)
	cc.cmd.Flags().Uint64Var(
		&cc.Capacity, "capacity", 0, "the channel's capacity in "+
			"satoshis",
	)
	cc.cmd.Flags().BoolVar(
		&cc.Initiator, "initiator", false, "whether our node was the "+
			"initiator (funder) of the channel",
	)
	multiFileName := fmt.Sprintf("results/fake-%s.backup",
		time.Now().Format("2006-01-02-15-04-05"))
	cc.cmd.Flags().StringVar(
		&cc.MultiFile, "multi_file", multiFileName, "the fake channel "+
			"backup file to create",
	)

	cc.rootKey = newRootKey(cc.cmd, "encrypting the backup")

	return cc.cmd
}

func (c *fakeChanBackupCommand) Execute(_ *cobra.Command, _ []string) error {
	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %v", err)
	}

	multiFile := chanbackup.NewMultiFile(c.MultiFile)
	keyRing := &lnd.HDKeyRing{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}

	// Parse channel point of channel to fake.
	chanOp, err := lnd.ParseOutpoint(c.ChannelPoint)
	if err != nil {
		return fmt.Errorf("error parsing channel point: %v", err)
	}

	// Now parse the remote node info.
	splitNodeInfo := strings.Split(c.NodeAddr, "@")
	if len(splitNodeInfo) != 2 {
		return fmt.Errorf("--remote_node_addr expected in format: " +
			"pubkey@host:port")
	}
	pubKeyBytes, err := hex.DecodeString(splitNodeInfo[0])
	if err != nil {
		return fmt.Errorf("could not parse pubkey hex string: %s", err)
	}
	nodePubkey, err := btcec.ParsePubKey(pubKeyBytes, btcec.S256())
	if err != nil {
		return fmt.Errorf("could not parse pubkey: %s", err)
	}
	addr, err := net.ResolveTCPAddr("tcp", splitNodeInfo[1])
	if err != nil {
		return fmt.Errorf("could not parse addr: %s", err)
	}

	// Parse the short channel ID.
	splitChanId := strings.Split(c.ShortChanID, "x")
	if len(splitChanId) != 3 {
		return fmt.Errorf("--short_channel_id expected in format: " +
			"<blockheight>x<transactionindex>x<outputindex>",
		)
	}
	blockHeight, err := strconv.ParseInt(splitChanId[0], 10, 32)
	if err != nil {
		return fmt.Errorf("could not parse block height: %s", err)
	}
	txIndex, err := strconv.ParseInt(splitChanId[1], 10, 32)
	if err != nil {
		return fmt.Errorf("could not parse transaction index: %s", err)
	}
	chanOutputIdx, err := strconv.ParseInt(splitChanId[2], 10, 32)
	if err != nil {
		return fmt.Errorf("could not parse output index: %s", err)
	}
	shortChanID := lnwire.ShortChannelID{
		BlockHeight: uint32(blockHeight),
		TxIndex:     uint32(txIndex),
		TxPosition:  uint16(chanOutputIdx),
	}

	// Is the outpoint and/or short channel ID correct?
	if uint32(chanOutputIdx) != chanOp.Index {
		return fmt.Errorf("output index of --short_channel_id must " +
			"be equal to index on --channelpoint")
	}

	// Create some fake channel config.
	chanCfg := channeldb.ChannelConfig{
		ChannelConstraints: channeldb.ChannelConstraints{
			DustLimit:        500,
			ChanReserve:      5000,
			MaxPendingAmount: 1,
			MinHTLC:          1,
			MaxAcceptedHtlcs: 200,
			CsvDelay:         144,
		},
		MultiSigKey: keychain.KeyDescriptor{
			PubKey: nodePubkey,
			KeyLocator: keychain.KeyLocator{
				Family: keychain.KeyFamilyMultiSig,
				Index:  0,
			},
		},
		RevocationBasePoint: keychain.KeyDescriptor{
			PubKey: nodePubkey,
			KeyLocator: keychain.KeyLocator{
				Family: keychain.KeyFamilyRevocationBase,
				Index:  0,
			},
		},
		PaymentBasePoint: keychain.KeyDescriptor{
			PubKey: nodePubkey,
			KeyLocator: keychain.KeyLocator{
				Family: keychain.KeyFamilyPaymentBase,
				Index:  0,
			},
		},
		DelayBasePoint: keychain.KeyDescriptor{
			PubKey: nodePubkey,
			KeyLocator: keychain.KeyLocator{
				Family: keychain.KeyFamilyDelayBase,
				Index:  0,
			},
		},
		HtlcBasePoint: keychain.KeyDescriptor{
			PubKey: nodePubkey,
			KeyLocator: keychain.KeyLocator{
				Family: keychain.KeyFamilyHtlcBase,
				Index:  0,
			},
		},
	}

	newMulti := chanbackup.Multi{
		Version: chanbackup.DefaultMultiVersion,
		StaticBackups: []chanbackup.Single{{
			Version:         chanbackup.DefaultSingleVersion,
			IsInitiator:     c.Initiator,
			ChainHash:       *chainParams.GenesisHash,
			FundingOutpoint: *chanOp,
			ShortChannelID:  shortChanID,
			RemoteNodePub:   nodePubkey,
			Addresses:       []net.Addr{addr},
			Capacity:        btcutil.Amount(c.Capacity),
			LocalChanCfg:    chanCfg,
			RemoteChanCfg:   chanCfg,
			ShaChainRootDesc: keychain.KeyDescriptor{
				PubKey: nodePubkey,
				KeyLocator: keychain.KeyLocator{
					Family: keychain.KeyFamilyRevocationRoot,
					Index:  1,
				},
			},
		}},
	}
	var packed bytes.Buffer
	err = newMulti.PackToWriter(&packed, keyRing)
	if err != nil {
		return fmt.Errorf("unable to multi-pack backups: %v", err)
	}

	return multiFile.UpdateAndSwap(packed.Bytes())
}
