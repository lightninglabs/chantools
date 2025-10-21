package main

import (
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/connmgr"
	"github.com/btcsuite/btcd/wire"
	"github.com/hasura/go-graphql-client"
	"github.com/lightninglabs/chantools/btc"
	"github.com/lightninglabs/chantools/cln"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/brontide"
	"github.com/lightningnetwork/lnd/fn/v2"
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

	APIURL            string
	AllPublicChannels bool

	TorProxy string

	HsmSecret string

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
	cc.cmd.Flags().BoolVar(
		&cc.AllPublicChannels, "all_public_channels", false,
		"query all public channels from the Amboss API and attempt "+
			"to trigger a force close for each of them",
	)
	cc.cmd.Flags().StringVar(
		&cc.TorProxy, "torproxy", "", "SOCKS5 proxy to use for Tor "+
			"connections (to .onion addresses)",
	)
	cc.cmd.Flags().StringVar(
		&cc.HsmSecret, "hsm_secret", "", "the hex encoded HSM secret "+
			"to use for deriving the node key for a CLN "+
			"node; obtain by running 'xxd -p -c32 "+
			"~/.lightning/bitcoin/hsm_secret'",
	)
	cc.rootKey = newRootKey(cc.cmd, "deriving the identity key")

	return cc.cmd
}

func (c *triggerForceCloseCommand) Execute(_ *cobra.Command, _ []string) error {
	var identityPriv *btcec.PrivateKey
	switch {
	case c.HsmSecret != "":
		secretBytes, err := hex.DecodeString(c.HsmSecret)
		if err != nil {
			return fmt.Errorf("error decoding HSM secret: %w", err)
		}

		var hsmSecret [32]byte
		copy(hsmSecret[:], secretBytes)

		_, identityPriv, err = cln.NodeKey(hsmSecret)
		if err != nil {
			return fmt.Errorf("error deriving identity key: %w",
				err)
		}

	default:
		extendedKey, err := c.rootKey.read()
		if err != nil {
			return fmt.Errorf("error reading root key: %w", err)
		}

		identityPath := lnd.IdentityPath(chainParams)
		child, _, _, err := lnd.DeriveKey(
			extendedKey, identityPath, chainParams,
		)
		if err != nil {
			return fmt.Errorf("could not derive identity key: %w",
				err)
		}
		identityPriv, err = child.ECPrivKey()
		if err != nil {
			return fmt.Errorf("could not get identity private "+
				"key: %w", err)
		}
	}

	api := newExplorerAPI(c.APIURL)
	switch {
	case c.ChannelPoint != "" && c.Peer != "":
		_, err := closeChannel(
			identityPriv, api, c.ChannelPoint, c.Peer, c.TorProxy,
		)
		return err

	case c.AllPublicChannels:
		client := graphql.NewClient(
			"https://api.amboss.space/graphql", nil,
		)
		ourNodeKey := hex.EncodeToString(
			identityPriv.PubKey().SerializeCompressed(),
		)

		log.Infof("Fetching public channels for node %s", ourNodeKey)
		channels, err := fetchChannels(client, ourNodeKey)
		if err != nil {
			return fmt.Errorf("error fetching channels: %w", err)
		}

		channels = fn.Filter(channels, func(c *gqChannel) bool {
			return c.ClosureInfo.ClosedHeight == 0
		})

		log.Infof("Found %d public open channels, attempting to force "+
			"close each of them", len(channels))

		var (
			pubKeys []string
			outputs []string
		)
		for idx, openChan := range channels {
			addr := pickAddr(openChan.Node2Info.Node.Addresses)
			peerAddr := fmt.Sprintf("%s@%s", openChan.Node2, addr)

			if c.TorProxy == "" &&
				strings.Contains(addr, ".onion") {

				log.Infof("Skipping channel %s with peer %s "+
					"because it is a Tor address and no "+
					"Tor proxy is configured",
					openChan.ChanPoint, peerAddr)
				continue
			}

			log.Infof("Attempting to force close channel %s with "+
				"peer %s (channel %d of %d)",
				openChan.ChanPoint, peerAddr, idx+1,
				len(channels))

			outputAddrs, err := closeChannel(
				identityPriv, api, openChan.ChanPoint,
				peerAddr, c.TorProxy,
			)
			if err != nil {
				log.Errorf("Error closing channel %s, "+
					"skipping and trying next one. "+
					"Reason: %v", openChan.ChanPoint, err)
				continue
			}

			pubKeys = append(pubKeys, openChan.Node2)
			outputs = append(outputs, outputAddrs...)
		}

		peersBytes := []byte(strings.Join(pubKeys, "\n"))
		outputsBytes := []byte(strings.Join(outputs, "\n"))

		fileName := fmt.Sprintf("%s/forceclose-peers-%s.txt",
			ResultsDir, time.Now().Format("2006-01-02"))
		log.Infof("Writing peers to %s", fileName)
		err = os.WriteFile(fileName, peersBytes, 0644)
		if err != nil {
			return fmt.Errorf("error writing peers to file: %w",
				err)
		}

		fileName = fmt.Sprintf("%s/forceclose-addresses-%s.txt",
			ResultsDir, time.Now().Format("2006-01-02"))
		log.Infof("Writing addresses to %s", fileName)
		return os.WriteFile(fileName, outputsBytes, 0644)

	default:
		return errors.New("either --channel_point and --peer or " +
			"--all_public_channels must be specified")
	}
}

func pickAddr(addrs []*gqAddress) string {
	// If there's only one address, we'll just return that one.
	if len(addrs) == 1 {
		return addrs[0].Address
	}

	// We'll pick the first address that is not a Tor address.
	for _, addr := range addrs {
		if !strings.Contains(addr.Address, ".onion") {
			return addr.Address
		}
	}

	// If all addresses are Tor addresses, we'll just return the first one.
	if len(addrs) > 0 {
		return addrs[0].Address
	}

	return ""
}

func closeChannel(identityPriv *btcec.PrivateKey, api *btc.ExplorerAPI,
	channelPoint, peer, torProxy string) ([]string, error) {

	identityECDH := &keychain.PrivKeyECDH{
		PrivKey: identityPriv,
	}

	outPoint, err := parseOutPoint(channelPoint)
	if err != nil {
		return nil, fmt.Errorf("error parsing channel point: %w", err)
	}

	err = requestForceCloseLnd(peer, torProxy, *outPoint, identityECDH)
	if err != nil {
		return nil, fmt.Errorf("error requesting force close: %w", err)
	}
	err = requestForceCloseCln(peer, torProxy, *outPoint, identityECDH)
	if err != nil {
		return nil, fmt.Errorf("error requesting force close: %w", err)
	}

	log.Infof("Message sent, waiting for force close transaction to " +
		"appear in mempool")

	channelAddress, err := api.Address(channelPoint)
	if err != nil {
		return nil, fmt.Errorf("error getting channel address: %w", err)
	}

	spends, err := api.Spends(channelAddress)
	if err != nil {
		return nil, fmt.Errorf("error getting spends: %w", err)
	}

	counter := 0
	for len(spends) == 0 {
		log.Infof("No spends found yet, waiting 5 seconds...")
		time.Sleep(5 * time.Second)
		spends, err = api.Spends(channelAddress)
		if err != nil {
			return nil, fmt.Errorf("error getting spends: %w", err)
		}

		counter++
		if counter >= 6 {
			log.Info("Waited 30 seconds, still no spends found, " +
				"re-triggering CLN request")
			err = requestForceCloseCln(
				peer, torProxy, *outPoint, identityECDH,
			)
			if err != nil {
				return nil, fmt.Errorf("error requesting "+
					"force close: %w", err)
			}
		}
		if counter >= 12 {
			return nil, errors.New("no spends found after 60 " +
				"seconds, aborting re-try loop")
		}
	}

	log.Infof("Found force close transaction %v", spends[0].TXID)
	log.Infof("You can now use the sweepremoteclosed command to sweep " +
		"the funds from the channel")

	outputAddrs := fn.Map(spends[0].Vout, func(v *btc.Vout) string {
		return v.ScriptPubkeyAddr
	})

	return outputAddrs, nil
}

func noiseDial(idKey keychain.SingleKeyECDH, lnAddr *lnwire.NetAddress,
	netCfg tor.Net, timeout time.Duration) (*brontide.Conn, error) {

	return brontide.Dial(idKey, lnAddr, timeout, netCfg.Dial)
}

func connectPeer(peerHost, torProxy string, identity keychain.SingleKeyECDH,
	dialTimeout time.Duration) (*peer.Brontide, func() error, error) {

	cleanup := func() error {
		return nil
	}

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
		return nil, cleanup, fmt.Errorf("error parsing peer address: "+
			"%w", err)
	}

	peerPubKey := peerAddr.IdentityKey

	log.Debugf("Attempting to dial resolved peer address %v",
		peerAddr.String())
	conn, err := noiseDial(identity, peerAddr, dialNet, dialTimeout)
	if err != nil {
		return nil, cleanup, fmt.Errorf("error dialing peer: %w", err)
	}

	cleanup = func() error {
		return conn.Close()
	}

	log.Infof("Attempting to establish p2p connection to peer %x, dial"+
		"timeout is %v", peerPubKey.SerializeCompressed(), dialTimeout)
	req := &connmgr.ConnReq{
		Addr:      peerAddr,
		Permanent: false,
	}
	p, channelDB, err := lnd.ConnectPeer(conn, req, chainParams, identity)
	if err != nil {
		return nil, cleanup, fmt.Errorf("error connecting to peer: %w",
			err)
	}

	cleanup = func() error {
		p.Disconnect(errors.New("done with peer"))
		if channelDB != nil {
			if err := channelDB.Close(); err != nil {
				log.Errorf("Error closing channel DB: %v", err)
			}
		}
		return conn.Close()
	}

	log.Infof("Connection established to peer %x",
		peerPubKey.SerializeCompressed())

	// We'll wait until the peer is active.
	select {
	case <-p.ActiveSignal():
	case <-p.QuitSignal():
		return nil, cleanup, fmt.Errorf("peer %x disconnected",
			peerPubKey.SerializeCompressed())
	}

	return p, cleanup, nil
}

func requestForceCloseLnd(peerHost, torProxy string, channelPoint wire.OutPoint,
	identity keychain.SingleKeyECDH) error {

	p, cleanup, err := connectPeer(
		peerHost, torProxy, identity, dialTimeout,
	)
	defer func() {
		_ = cleanup()
	}()

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

	// Wait a few seconds to give the peer time to process the message.
	time.Sleep(5 * time.Second)

	return nil
}

func requestForceCloseCln(peerHost, torProxy string, channelPoint wire.OutPoint,
	identity keychain.SingleKeyECDH) error {

	p, cleanup, err := connectPeer(
		peerHost, torProxy, identity, dialTimeout,
	)
	defer func() {
		_ = cleanup()
	}()

	if err != nil {
		return fmt.Errorf("error connecting to peer: %w", err)
	}

	channelID := lnwire.NewChanIDFromOutPoint(channelPoint)

	// Channel ID (32 byte) + u16 for the data length (which will be 0).
	data := make([]byte, 34)
	copy(data[:32], channelID[:])

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

	// Wait a few seconds to give the peer time to process the message.
	time.Sleep(5 * time.Second)

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
