package main

import (
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lightninglabs/chantools/cln"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/spf13/cobra"
)

var (
	hexPubKeyLen = hex.EncodedLen(btcec.PubKeyBytesLenCompressed)
)

type clnSweepRemoteClosedCommand struct {
	HsmSecret      string
	PeerPubKeys    string
	RecoveryWindow uint32
	APIURL         string
	Publish        bool
	SweepAddr      string
	FeeRate        uint32

	cmd *cobra.Command
}

func newClnSweepRemoteClosedCommand() *cobra.Command {
	cc := &clnSweepRemoteClosedCommand{}
	cc.cmd = &cobra.Command{
		Use: "clnsweepremoteclosed",
		Short: "CLN only: Go through all the addresses that could " +
			"have funds of channels that were force-closed by " +
			"the remote party. A public block explorer is " +
			"queried for each address and if any balance is " +
			"found, all funds are swept to a given address",
		Long: `This command helps CLN users sweep funds that are in 
outputs of channels that were force-closed by the remote party. By manually
contacting the remote peers and asking them to force-close the channels, the
funds can be swept after the force-close transaction was confirmed.

Supported remote force-closed channel types are:
 - STATIC_REMOTE_KEY (a.k.a. tweakless channels)
 - ANCHOR (a.k.a. anchor output channels)
`,
		Example: `chantools clnsweepremoteclosed \
	--hsm_secret xxyyzz... \
	--peers 02aabbccdd...,02eeffgg... \
	--recoverywindow 300 \
	--feerate 20 \
	--sweepaddr bc1q..... \
  	--publish`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.HsmSecret, "hsm_secret", "", "the hex encoded HSM secret "+
			"to use for deriving the keys for a CLN "+
			"node; obtain by running 'xxd -p -c32 "+
			"~/.lightning/bitcoin/hsm_secret'",
	)
	cc.cmd.Flags().StringVar(
		&cc.PeerPubKeys, "peers", "", "comma separated list of "+
			"hex encoded public keys of the remote peers "+
			"to recover funds from",
	)
	cc.cmd.Flags().Uint32Var(
		&cc.RecoveryWindow, "recoverywindow",
		sweepRemoteClosedDefaultRecoveryWindow, "number of keys to "+
			"scan per channel, should be higher than the "+
			"approximate number of channels that ever existed on "+
			"the node",
	)
	cc.cmd.Flags().StringVar(
		&cc.APIURL, "apiurl", defaultAPIURL, "API URL to use (must "+
			"be esplora compatible)",
	)
	cc.cmd.Flags().BoolVar(
		&cc.Publish, "publish", false, "publish sweep TX to the chain "+
			"API instead of just printing the TX",
	)
	cc.cmd.Flags().StringVar(
		&cc.SweepAddr, "sweepaddr", "", "address to recover the funds "+
			"to; specify '"+lnd.AddressDeriveFromWallet+"' to "+
			"derive a new address from the seed automatically",
	)
	cc.cmd.Flags().Uint32Var(
		&cc.FeeRate, "feerate", defaultFeeSatPerVByte, "fee rate to "+
			"use for the sweep transaction in sat/vByte",
	)

	return cc.cmd
}

func (c *clnSweepRemoteClosedCommand) Execute(_ *cobra.Command,
	_ []string) error {

	// Make sure sweep addr is set.
	err := lnd.CheckAddress(
		c.SweepAddr, chainParams, true, "sweep", lnd.AddrTypeP2WKH,
		lnd.AddrTypeP2TR,
	)
	if err != nil {
		return err
	}

	// Set default values.
	if c.RecoveryWindow == 0 {
		c.RecoveryWindow = sweepRemoteClosedDefaultRecoveryWindow
	}
	if c.FeeRate == 0 {
		c.FeeRate = defaultFeeSatPerVByte
	}

	if len(c.HsmSecret) != hex.EncodedLen(32) {
		return fmt.Errorf("invalid HSM secret length")
	}

	secretBytes, err := hex.DecodeString(c.HsmSecret)
	if err != nil {
		return fmt.Errorf("error decoding HSM secret: %w", err)
	}

	var hsmSecret [32]byte
	copy(hsmSecret[:], secretBytes)

	if c.PeerPubKeys == "" || len(c.PeerPubKeys)%hexPubKeyLen != 0 {
		return fmt.Errorf("invalid peer public keys, must be a " +
			"comma separated list of hex encoded public keys")
	}

	var pubKeys []*btcec.PublicKey
	for _, pubKeyHex := range strings.Split(c.PeerPubKeys, ",") {
		pkHex, err := hex.DecodeString(pubKeyHex)
		if err != nil {
			return fmt.Errorf("error decoding peer public key "+
				"hex %s: %w", pubKeyHex, err)
		}

		pk, err := btcec.ParsePubKey(pkHex)
		if err != nil {
			return fmt.Errorf("error parsing peer public key "+
				"hex %s: %w", pubKeyHex, err)
		}

		pubKeys = append(pubKeys, pk)
	}

	return clnSweepRemoteClosed(
		hsmSecret, pubKeys, c.APIURL, c.SweepAddr, c.RecoveryWindow,
		c.FeeRate, c.Publish,
	)
}

func clnSweepRemoteClosed(hsmSecret [32]byte, pubKeys []*btcec.PublicKey,
	apiURL, sweepAddr string, recoveryWindow uint32, feeRate uint32,
	publish bool) error {

	var (
		targets []*targetAddr
		api     = newExplorerAPI(apiURL)
	)
	for _, pubKey := range pubKeys {
		for index := range recoveryWindow {
			privKey, err := cln.PaymentBasePointSecret(
				hsmSecret, pubKey, uint64(index),
			)
			if err != nil {
				return fmt.Errorf("could not derive private "+
					"key: %w", err)
			}

			foundTargets, err := queryAddressBalances(
				privKey.PubKey(), &keychain.KeyDescriptor{
					PubKey: privKey.PubKey(),
					KeyLocator: keychain.KeyLocator{
						Family: keychain.KeyFamilyPaymentBase,
						Index:  index,
					},
				}, api,
			)
			if err != nil {
				return fmt.Errorf("could not query API for "+
					"addresses with funds: %w", err)
			}
			targets = append(targets, foundTargets...)
		}
	}

	log.Infof("Found %d addresses with funds to sweep.", len(targets))

	return nil
}
