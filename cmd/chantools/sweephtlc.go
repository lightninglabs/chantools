package main

import (
	"errors"
	"fmt"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/spf13/cobra"
)

// sweepHtlcCommand holds the CLI options and dependencies for sweephtlc.
type sweepHtlcCommand struct {
	// ChannelDB is the lnd channel.db path used to recover HTLC metadata.
	ChannelDB string

	// Outpoints is the comma separated list of exact HTLC outputs to sweep.
	Outpoints string

	// CommitPoint is an optional commitment point override for DLP cases.
	CommitPoint string

	// APIURL is the Esplora-compatible chain API endpoint.
	APIURL string

	// Publish controls whether the signed sweep transaction is broadcast.
	Publish bool

	// SweepAddr is the destination address for recovered funds.
	SweepAddr string

	// FeeRate is the target sweep fee rate in sat/vByte.
	FeeRate uint32

	// rootKey loads the wallet root key used for HTLC signing.
	rootKey *rootKey

	// cmd is the cobra command bound to this option set.
	cmd *cobra.Command
}

// newSweepHtlcCommand creates the sweephtlc cobra command.
func newSweepHtlcCommand() *cobra.Command {
	cc := &sweepHtlcCommand{}
	cc.cmd = &cobra.Command{
		Use:   "sweephtlc",
		Short: "Sweep channel HTLC outputs by matching outpoints against channel.db",
		Long: `Sweep channel HTLC outputs by taking exact on-chain outpoints,
finding the corresponding channel in channel.db, reconstructing candidate HTLC
scripts and signing the matching spend.

The first supported spend path is an outgoing HTLC on the remote party's
commitment transaction after CLTV timeout. This is the direct timeout spend used
when the remote party force-closes with an HTLC we offered.

By default the command only prints the raw transaction. Use --publish to publish
through the configured Esplora-compatible API.`,
		Example: `chantools sweephtlc \
	--channeldb ~/.lnd/data/graph/mainnet/channel.db \
	--outpoints aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:3,aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:4 \
	--commitpoint 0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798 \
	--sweepaddr bc1q..... \
	--feerate 1`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.ChannelDB, "channeldb", "", "lnd channel.db file to read "+
			"channel state from",
	)
	cc.cmd.Flags().StringVar(
		&cc.Outpoints, "outpoints", "", "comma separated HTLC outpoints "+
			"to sweep, in txid:index format",
	)
	cc.cmd.Flags().StringVar(
		&cc.CommitPoint, "commitpoint", "", "optional commitment point "+
			"override to try when reconstructing HTLC scripts",
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
		&cc.FeeRate, "feerate", 1, "fee rate to "+
			"use for the sweep transaction in sat/vByte",
	)

	cc.rootKey = newRootKey(cc.cmd, "signing HTLC sweep transaction")

	return cc.cmd
}

// Execute runs the sweephtlc command.
func (c *sweepHtlcCommand) Execute(_ *cobra.Command, _ []string) error {
	if c.ChannelDB == "" {
		return errors.New("channel DB is required")
	}
	if c.Outpoints == "" {
		return errors.New("at least one outpoint is required")
	}
	if c.FeeRate == 0 {
		c.FeeRate = 1
	}

	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %w", err)
	}

	db, _, err := lnd.OpenDB(c.ChannelDB, true)
	if err != nil {
		return fmt.Errorf("error opening channel DB: %w", err)
	}
	defer func() { _ = db.Close() }()

	var commitPointOverride *btcec.PublicKey
	if strings.TrimSpace(c.CommitPoint) != "" {
		commitPointOverride, err = parsePubKey(c.CommitPoint)
		if err != nil {
			return fmt.Errorf("error parsing commit point: %w", err)
		}
	}

	api := newExplorerAPI(c.APIURL)
	targets, err := fetchSweepHtlcTargets(api, c.Outpoints)
	if err != nil {
		return err
	}

	matches, err := findSweepHtlcMatches(
		db.ChannelStateDB(), targets, commitPointOverride,
	)
	if err != nil {
		return err
	}

	return sweepMatchedHtlcs(
		extendedKey, api, matches, c.SweepAddr, c.FeeRate, c.Publish,
	)
}
