package main

import (
	"fmt"
	"net"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/connmgr"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightninglabs/chantools/btc"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/brontide"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lncfg"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/tor"
	"github.com/spf13/cobra"
)

var (
	dialTimeout = time.Minute
)

type triggerForceCloseCommand struct {
	Peer         string
	ChannelPoint string

	APIURL string

	rootKey *rootKey
	cmd     *cobra.Command
}

func newTriggerForceCloseCommand() *cobra.Command {
	cc := &triggerForceCloseCommand{}
	cc.cmd = &cobra.Command{
		Use: "triggerforceclose",
		Short: "Connect to a peer and send a custom message to " +
			"trigger a force close of the specified channel",
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

	peerAddr, err := lncfg.ParseLNAddressString(
		c.Peer, "9735", net.ResolveTCPAddr,
	)
	if err != nil {
		return fmt.Errorf("error parsing peer address: %w", err)
	}

	outPoint, err := parseOutPoint(c.ChannelPoint)
	if err != nil {
		return fmt.Errorf("error parsing channel point: %w", err)
	}
	channelID := lnwire.NewChanIDFromOutPoint(outPoint)

	conn, err := noiseDial(
		identityECDH, peerAddr, &tor.ClearNet{}, dialTimeout,
	)
	if err != nil {
		return fmt.Errorf("error dialing peer: %w", err)
	}

	log.Infof("Attempting to connect to peer %x, dial timeout is %v",
		pubKey.SerializeCompressed(), dialTimeout)
	req := &connmgr.ConnReq{
		Addr:      peerAddr,
		Permanent: false,
	}
	p, err := lnd.ConnectPeer(conn, req, chainParams, identityECDH)
	if err != nil {
		return fmt.Errorf("error connecting to peer: %w", err)
	}

	log.Infof("Connection established to peer %x",
		pubKey.SerializeCompressed())

	// We'll wait until the peer is active.
	select {
	case <-p.ActiveSignal():
	case <-p.QuitSignal():
		return fmt.Errorf("peer %x disconnected",
			pubKey.SerializeCompressed())
	}

	// Channel ID (32 byte) + u16 for the data length (which will be 0).
	data := make([]byte, 34)
	copy(data[:32], channelID[:])

	log.Infof("Sending channel error message to peer to trigger force "+
		"close of channel %v", c.ChannelPoint)

	_ = lnwire.SetCustomOverrides([]uint16{lnwire.MsgError})
	msg, err := lnwire.NewCustom(lnwire.MsgError, data)
	if err != nil {
		return err
	}

	err = p.SendMessageLazy(true, msg)
	if err != nil {
		return fmt.Errorf("error sending message: %w", err)
	}

	log.Infof("Message sent, waiting for force close transaction to " +
		"appear in mempool")

	api := &btc.ExplorerAPI{BaseURL: c.APIURL}
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
