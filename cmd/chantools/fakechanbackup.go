package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"io/ioutil"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/wire"
	"github.com/gogo/protobuf/jsonpb"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/chanbackup"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/tor"
	"github.com/spf13/cobra"
)

type fakeChanBackupCommand struct {
	NodeAddr     string
	ChannelPoint string
	ShortChanID  string
	Capacity     uint64

	FromChannelGraph string

	MultiFile string

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
the transaction output of our part of the funds (see rescueclosed command).

There are two versions of this command: The first one is to create a fake
backup for a single channel where all flags (except --from_channel_graph) need
to be set. This is the easiest to use since it only relies on data that is
publicly available (for example on 1ml.com) but involves more manual work.
The second version of the command only takes the --from_channel_graph and
--multi_file flags and tries to assemble all channels found in the public
network graph (must be provided in the JSON format that the 
'lncli describegraph' command returns) into a fake backup file. This is the
most convenient way to use this command but requires one to have a fully synced
lnd node.

Any fake channel backup _needs_ to be used with the custom fork of lnd
specifically built for this purpose: https://github.com/guggero/lnd/releases
Also the debuglevel must be set to debug (lnd.conf, set 'debuglevel=debug') when
running the above lnd for it to produce the correct log file that will be needed
for the rescueclosed command.
`,
		Example: `chantools fakechanbackup \
	--capacity 123456 \
	--channelpoint f39310xxxxxxxxxx:1 \
	--remote_node_addr 022c260xxxxxxxx@213.174.150.1:9735 \
	--short_channel_id 566222x300x1 \
	--multi_file fake.backup

chantools fakechanbackup --from_channel_graph lncli_describegraph.json \
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
	cc.cmd.Flags().StringVar(
		&cc.FromChannelGraph, "from_channel_graph", "", "the full "+
			"LN channel graph in the JSON format that the "+
			"'lncli describegraph' returns",
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
		return fmt.Errorf("error reading root key: %w", err)
	}

	multiFile := chanbackup.NewMultiFile(c.MultiFile)
	keyRing := &lnd.HDKeyRing{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}

	if c.FromChannelGraph != "" {
		graphBytes, err := ioutil.ReadFile(c.FromChannelGraph)
		if err != nil {
			return fmt.Errorf("error reading graph JSON file %s: "+
				"%v", c.FromChannelGraph, err)
		}
		graph := &lnrpc.ChannelGraph{}
		err = jsonpb.UnmarshalString(string(graphBytes), graph)
		if err != nil {
			return fmt.Errorf("error parsing graph JSON: %w", err)
		}

		return backupFromGraph(graph, keyRing, multiFile)
	}

	// Parse channel point of channel to fake.
	chanOp, err := lnd.ParseOutpoint(c.ChannelPoint)
	if err != nil {
		return fmt.Errorf("error parsing channel point: %w", err)
	}

	// Now parse the remote node info.
	splitNodeInfo := strings.Split(c.NodeAddr, "@")
	if len(splitNodeInfo) != 2 {
		return fmt.Errorf("--remote_node_addr expected in format: " +
			"pubkey@host:port")
	}
	pubKeyBytes, err := hex.DecodeString(splitNodeInfo[0])
	if err != nil {
		return fmt.Errorf("could not parse pubkey hex string: %w", err)
	}
	nodePubkey, err := btcec.ParsePubKey(pubKeyBytes)
	if err != nil {
		return fmt.Errorf("could not parse pubkey: %w", err)
	}
	host, portStr, err := net.SplitHostPort(splitNodeInfo[1])
	if err != nil {
		return fmt.Errorf("could not split host and port: %w", err)
	}

	var addr net.Addr
	if tor.IsOnionHost(host) {
		port, err := strconv.Atoi(portStr)
		if err != nil {
			return fmt.Errorf("could not parse port: %w", err)
		}
		addr = &tor.OnionAddr{
			OnionService: host,
			Port:         port,
		}
	} else {
		addr, err = net.ResolveTCPAddr("tcp", splitNodeInfo[1])
		if err != nil {
			return fmt.Errorf("could not parse addr: %w", err)
		}
	}

	// Parse the short channel ID.
	splitChanID := strings.Split(c.ShortChanID, "x")
	if len(splitChanID) != 3 {
		return fmt.Errorf("--short_channel_id expected in format: " +
			"<blockheight>x<transactionindex>x<outputindex>",
		)
	}
	blockHeight, err := strconv.ParseInt(splitChanID[0], 10, 32)
	if err != nil {
		return fmt.Errorf("could not parse block height: %w", err)
	}
	txIndex, err := strconv.ParseInt(splitChanID[1], 10, 32)
	if err != nil {
		return fmt.Errorf("could not parse transaction index: %w", err)
	}
	chanOutputIdx, err := strconv.ParseInt(splitChanID[2], 10, 32)
	if err != nil {
		return fmt.Errorf("could not parse output index: %w", err)
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

	singles := []chanbackup.Single{newSingle(
		*chanOp, shortChanID, nodePubkey, []net.Addr{addr},
		btcutil.Amount(c.Capacity),
	)}
	return writeBackups(singles, keyRing, multiFile)
}

func backupFromGraph(graph *lnrpc.ChannelGraph, keyRing *lnd.HDKeyRing,
	multiFile *chanbackup.MultiFile) error {

	// Since we have the master root key, we can find out our local node's
	// identity pubkey by just deriving it.
	nodePubKey, err := keyRing.NodePubKey()
	if err != nil {
		return fmt.Errorf("error deriving node pubkey: %w", err)
	}
	nodePubKeyStr := hex.EncodeToString(nodePubKey.SerializeCompressed())

	// Let's now find all channels in the graph that our node is part of.
	channels := lnd.AllNodeChannels(graph, nodePubKeyStr)

	// Let's create a single backup entry for each channel.
	singles := make([]chanbackup.Single, len(channels))
	for idx, channel := range channels {
		var peerPubKeyStr string
		if channel.Node1Pub == nodePubKeyStr {
			peerPubKeyStr = channel.Node2Pub
		} else {
			peerPubKeyStr = channel.Node1Pub
		}

		peerPubKeyBytes, err := hex.DecodeString(peerPubKeyStr)
		if err != nil {
			return fmt.Errorf("error parsing hex: %w", err)
		}
		peerPubKey, err := btcec.ParsePubKey(peerPubKeyBytes)
		if err != nil {
			return fmt.Errorf("error parsing pubkey: %w", err)
		}

		peer, err := lnd.FindNode(graph, peerPubKeyStr)
		if err != nil {
			return err
		}
		peerAddresses := make([]net.Addr, len(peer.Addresses))
		for idx, peerAddr := range peer.Addresses {
			var err error
			if strings.Contains(peerAddr.Addr, ".onion") {
				peerAddresses[idx], err = tor.ParseAddr(
					peerAddr.Addr, "",
				)
				if err != nil {
					return fmt.Errorf("error parsing "+
						"tor address: %w", err)
				}

				continue
			}
			peerAddresses[idx], err = net.ResolveTCPAddr(
				"tcp", peerAddr.Addr,
			)
			if err != nil {
				return fmt.Errorf("could not parse addr: %w",
					err)
			}
		}

		shortChanID := lnwire.NewShortChanIDFromInt(channel.ChannelId)
		chanOp, err := lnd.ParseOutpoint(channel.ChanPoint)
		if err != nil {
			return fmt.Errorf("error parsing channel point: %w",
				err)
		}

		singles[idx] = newSingle(
			*chanOp, shortChanID, peerPubKey, peerAddresses,
			btcutil.Amount(channel.Capacity),
		)
	}

	return writeBackups(singles, keyRing, multiFile)
}

func writeBackups(singles []chanbackup.Single, keyRing keychain.KeyRing,
	multiFile *chanbackup.MultiFile) error {

	newMulti := chanbackup.Multi{
		Version:       chanbackup.DefaultMultiVersion,
		StaticBackups: singles,
	}
	var packed bytes.Buffer
	err := newMulti.PackToWriter(&packed, keyRing)
	if err != nil {
		return fmt.Errorf("unable to multi-pack backups: %w", err)
	}

	return multiFile.UpdateAndSwap(packed.Bytes())
}

func newSingle(fundingOutPoint wire.OutPoint, shortChanID lnwire.ShortChannelID,
	nodePubKey *btcec.PublicKey, addrs []net.Addr,
	capacity btcutil.Amount) chanbackup.Single {

	return chanbackup.Single{
		Version:         chanbackup.DefaultSingleVersion,
		IsInitiator:     true,
		ChainHash:       *chainParams.GenesisHash,
		FundingOutpoint: fundingOutPoint,
		ShortChannelID:  shortChanID,
		RemoteNodePub:   nodePubKey,
		Addresses:       addrs,
		Capacity:        capacity,
		LocalChanCfg:    fakeChanCfg(nodePubKey),
		RemoteChanCfg:   fakeChanCfg(nodePubKey),
		ShaChainRootDesc: keychain.KeyDescriptor{
			PubKey: nodePubKey,
			KeyLocator: keychain.KeyLocator{
				Family: keychain.KeyFamilyRevocationRoot,
				Index:  1,
			},
		},
	}
}

func fakeChanCfg(nodePubkey *btcec.PublicKey) channeldb.ChannelConfig {
	return channeldb.ChannelConfig{
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
}
