package main

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/connmgr"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/brontide"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lncfg"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/peer"
	"github.com/lightningnetwork/lnd/tor"
	"github.com/spf13/cobra"
)

var (
	dialTimeout = time.Minute

	defaultTorDNSHostPort = "soa.nodes.lightning.directory:53"
)

type triggerForceCloseCommand struct {
	Peer         string
	ChannelPoint string

	APIURL string

	TorProxy string

	rootKey *rootKey
	cmd     *cobra.Command
}

func newTriggerForceCloseCommand() *cobra.Command {
	cc := &triggerForceCloseCommand{}
	cc.cmd = &cobra.Command{
		Use: "triggerforceclose",
		Short: "Connect to a Lightning Network peer and send " +
			"specific messages to trigger a force close of the " +
			"specified channel",
		Long: `Asks the specified remote peer to force close a specific
channel by first sending a channel re-establish message, and if that doesn't
work, a custom error message (in case the peer is a specific version of CLN that
does not properly respond to a Data Loss Protection re-establish message).'`,
		Example: `chantools triggerforceclose \
	--peer 03abce...@xx.yy.zz.aa:9735 \
	--channel_point abcdef01234...:x`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.Peer, "peer", "", "remote peer address "+
			"(<pubkey>@<host>[:<port>])",
	)
	cc.cmd.Flags().StringVar(
		&cc.ChannelPoint, "channel_point", "", "funding transaction "+
			"outpoint of the channel to trigger the force close "+
			"of (<txid>:<txindex>)",
	)
	cc.cmd.Flags().StringVar(
		&cc.APIURL, "apiurl", defaultAPIURL, "API URL to use (must "+
			"be esplora compatible)",
	)
	cc.cmd.Flags().StringVar(
		&cc.TorProxy, "torproxy", "", "SOCKS5 proxy to use for Tor "+
			"connections (to .onion addresses)",
	)
	cc.rootKey = newRootKey(cc.cmd, "deriving the identity key")

	return cc.cmd
}

func (c *triggerForceCloseCommand) Execute(_ *cobra.Command, _ []string) error {
	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %w", err)
	}

	identityPath := lnd.IdentityPath(chainParams)
	child, pubKey, _, err := lnd.DeriveKey(
		extendedKey, identityPath, chainParams,
	)
	if err != nil {
		return fmt.Errorf("could not derive identity key: %w", err)
	}
	identityPriv, err := child.ECPrivKey()
	if err != nil {
		return fmt.Errorf("could not get identity private key: %w", err)
	}
	identityECDH := &keychain.PrivKeyECDH{
		PrivKey: identityPriv,
	}

	outPoint, err := parseOutPoint(c.ChannelPoint)
	if err != nil {
		return fmt.Errorf("error parsing channel point: %w", err)
	}

	err = requestForceClose(
		c.Peer, c.TorProxy, pubKey, *outPoint, identityECDH,
	)
	if err != nil {
		return fmt.Errorf("error requesting force close: %w", err)
	}

	log.Infof("Message sent, waiting for force close transaction to " +
		"appear in mempool")

	api := newExplorerAPI(c.APIURL)
	channelAddress, err := api.Address(c.ChannelPoint)
	if err != nil {
		return fmt.Errorf("error getting channel address: %w", err)
	}

	spends, err := api.Spends(channelAddress)
	if err != nil {
		return fmt.Errorf("error getting spends: %w", err)
	}
	for len(spends) == 0 {
		log.Infof("No spends found yet, waiting 5 seconds...")
		time.Sleep(5 * time.Second)
		spends, err = api.Spends(channelAddress)
		if err != nil {
			return fmt.Errorf("error getting spends: %w", err)
		}
	}

	log.Infof("Found force close transaction %v", spends[0].TXID)
	log.Infof("You can now use the sweepremoteclosed command to sweep " +
		"the funds from the channel")

	return nil
}

func noiseDial(idKey keychain.SingleKeyECDH, lnAddr *lnwire.NetAddress,
	netCfg tor.Net, timeout time.Duration) (*brontide.Conn, error) {

	return brontide.Dial(idKey, lnAddr, timeout, netCfg.Dial)
}

func connectPeer(peerHost, torProxy string, peerPubKey *btcec.PublicKey,
	identity keychain.SingleKeyECDH,
	dialTimeout time.Duration) (*peer.Brontide, error) {

	var dialNet tor.Net = &tor.ClearNet{}
	if torProxy != "" {
		dialNet = &tor.ProxyNet{
			SOCKS:                       torProxy,
			DNS:                         defaultTorDNSHostPort,
			StreamIsolation:             false,
			SkipProxyForClearNetTargets: true,
		}
	}

	log.Debugf("Attempting to resolve peer address %v", peerHost)
	peerAddr, err := lncfg.ParseLNAddressString(
		peerHost, "9735", dialNet.ResolveTCPAddr,
	)
	if err != nil {
		return nil, fmt.Errorf("error parsing peer address: %w", err)
	}

	log.Debugf("Attempting to dial resolved peer address %v",
		peerAddr.String())
	conn, err := noiseDial(identity, peerAddr, dialNet, dialTimeout)
	if err != nil {
		return nil, fmt.Errorf("error dialing peer: %w", err)
	}

	log.Infof("Attempting to establish p2p connection to peer %x, dial"+
		"timeout is %v", peerPubKey.SerializeCompressed(), dialTimeout)
	req := &connmgr.ConnReq{
		Addr:      peerAddr,
		Permanent: false,
	}
	p, err := lnd.ConnectPeer(conn, req, chainParams, identity)
	if err != nil {
		return nil, fmt.Errorf("error connecting to peer: %w", err)
	}

	log.Infof("Connection established to peer %x",
		peerPubKey.SerializeCompressed())

	// We'll wait until the peer is active.
	select {
	case <-p.ActiveSignal():
	case <-p.QuitSignal():
		return nil, fmt.Errorf("peer %x disconnected",
			peerPubKey.SerializeCompressed())
	}

	return p, nil
}

func requestForceClose(peerHost, torProxy string, peerPubKey *btcec.PublicKey,
	channelPoint wire.OutPoint, identity keychain.SingleKeyECDH) error {

	p, err := connectPeer(
		peerHost, torProxy, peerPubKey, identity, dialTimeout,
	)
	if err != nil {
		return fmt.Errorf("error connecting to peer: %w", err)
	}

	channelID := lnwire.NewChanIDFromOutPoint(channelPoint)

	// Channel ID (32 byte) + u16 for the data length (which will be 0).
	data := make([]byte, 34)
	copy(data[:32], channelID[:])

	log.Infof("Sending channel re-establish to peer to trigger force "+
		"close of channel %v", channelPoint)

	err = p.SendMessageLazy(true, &lnwire.ChannelReestablish{
		ChanID: channelID,
	})
	if err != nil {
		return err
	}

	log.Infof("Sending channel error message to peer to trigger force "+
		"close of channel %v", channelPoint)

	_ = lnwire.SetCustomOverrides([]uint16{
		lnwire.MsgError, lnwire.MsgChannelReestablish,
	})
	msg, err := lnwire.NewCustom(lnwire.MsgError, data)
	if err != nil {
		return err
	}

	err = p.SendMessageLazy(true, msg)
	if err != nil {
		return fmt.Errorf("error sending message: %w", err)
	}

	return nil
}

func parseOutPoint(s string) (*wire.OutPoint, error) {
	split := strings.Split(s, ":")
	if len(split) != 2 || len(split[0]) == 0 || len(split[1]) == 0 {
		return nil, fmt.Errorf("invalid channel point format: %v", s)
	}

	index, err := strconv.ParseInt(split[1], 10, 64)
	if err != nil {
		return nil, fmt.Errorf("unable to decode output index: %w", err)
	}

	txid, err := chainhash.NewHashFromStr(split[0])
	if err != nil {
		return nil, fmt.Errorf("unable to parse hex string: %w", err)
	}

	return &wire.OutPoint{
		Hash:  *txid,
		Index: uint32(index),
	}, nil
}
