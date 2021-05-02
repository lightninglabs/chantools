package main

import (
	"bytes"
	"fmt"

	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/psbt"
	"github.com/guggero/chantools/lnd"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/lnwallet/chainfee"
	"github.com/spf13/cobra"
)

const (
	MaxChannelLookup = 5000

	// MultiSigWitnessSize 222 bytes
	//	- NumberOfWitnessElements: 1 byte
	//	- NilLength: 1 byte
	//	- sigAliceLength: 1 byte
	//	- sigAlice: 73 bytes
	//	- sigBobLength: 1 byte
	//	- sigBob: 73 bytes
	//	- WitnessScriptLength: 1 byte
	//	- WitnessScript (MultiSig)
	MultiSigWitnessSize = 1 + 1 + 1 + 73 + 1 + 73 + 1 + input.MultiSigSize
)

var (
	PsbtKeyTypeOutputMissingSigPubkey = []byte{0xcc}
)

type rescueFundingCommand struct {
	ChannelDB         string `long:"channeldb" description:"The lnd channel.db file to rescue a channel from. Must contain the pending channel specified with --channelpoint."`
	ChannelPoint      string `long:"channelpoint" description:"The funding transaction outpoint of the channel to rescue (<txid>:<txindex>) as it is recorded in the DB."`
	ConfirmedOutPoint string `long:"confirmedchannelpoint" description:"The channel outpoint that got confirmed on chain (<txid>:<txindex>). Normally this is the same as the --channelpoint so it will be set to that value if this is left empty."`
	SweepAddr         string
	FeeRate           uint16

	rootKey *rootKey
	cmd     *cobra.Command
}

func newRescueFundingCommand() *cobra.Command {
	cc := &rescueFundingCommand{}
	cc.cmd = &cobra.Command{
		Use: "rescuefunding",
		Short: "Rescue funds locked in a funding multisig output that " +
			"never resulted in a proper channel; this is the " +
			"command the initiator of the channel needs to run",
		Long: `This is part 1 of a two phase process to rescue a channel
funding output that was created on chain by accident but never resulted in a
proper channel and no commitment transactions exist to spend the funds locked in
the 2-of-2 multisig.

**You need the cooperation of the channel partner (remote node) for this to
work**! They need to run the second command of this process: signrescuefunding

If successful, this will create a PSBT that then has to be sent to the channel
partner (remote node operator).`,
		Example: `chantools rescuefunding \
	--channeldb ~/.lnd/data/graph/mainnet/channel.db \
	--channelpoint xxxxxxx:xx \
	--sweepaddr bc1qxxxxxxxxx \
	--feerate 10`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.ChannelDB, "channeldb", "", "lnd channel.db file to "+
			"rescue a channel from; must contain the pending "+
			"channel specified with --channelpoint",
	)
	cc.cmd.Flags().StringVar(
		&cc.ChannelPoint, "channelpoint", "", "funding transaction "+
			"outpoint of the channel to rescue (<txid>:<txindex>) "+
			"as it is recorded in the DB",
	)
	cc.cmd.Flags().StringVar(
		&cc.ConfirmedOutPoint, "confirmedchannelpoint", "", "channel "+
			"outpoint that got confirmed on chain "+
			"(<txid>:<txindex>); normally this is the same as the "+
			"--channelpoint so it will be set to that value if"+
			"this is left empty",
	)
	cc.cmd.Flags().StringVar(
		&cc.SweepAddr, "sweepaddr", "", "address to sweep the funds to",
	)
	cc.cmd.Flags().Uint16Var(
		&cc.FeeRate, "feerate", defaultFeeSatPerVByte, "fee rate to "+
			"use for the sweep transaction in sat/vByte",
	)

	cc.rootKey = newRootKey(cc.cmd, "deriving keys")

	return cc.cmd
}

func (c *rescueFundingCommand) Execute(_ *cobra.Command, _ []string) error {
	var (
		chainOp *wire.OutPoint
	)

	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %v", err)
	}

	signer := &lnd.Signer{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}

	// Check that we have a channel DB.
	if c.ChannelDB == "" {
		return fmt.Errorf("channel DB is required")
	}
	db, err := lnd.OpenDB(c.ChannelDB, true)
	if err != nil {
		return fmt.Errorf("error opening rescue DB: %v", err)
	}

	// Parse channel point of channel to rescue as known to the DB.
	dbOp, err := lnd.ParseOutpoint(c.ChannelPoint)
	if err != nil {
		return fmt.Errorf("error parsing channel point: %v", err)
	}

	// Parse channel point of channel to rescue as confirmed on chain (if
	// different).
	if len(c.ConfirmedOutPoint) == 0 {
		chainOp = dbOp
	} else {
		chainOp, err = lnd.ParseOutpoint(c.ConfirmedOutPoint)
		if err != nil {
			return fmt.Errorf("error parsing confirmed channel "+
				"point: %v", err)
		}
	}

	// Make sure the sweep addr is a P2WKH address so we can do accurate
	// fee estimation.
	sweepScript, err := lnd.GetP2WPKHScript(c.SweepAddr, chainParams)
	if err != nil {
		return fmt.Errorf("error parsing sweep addr: %v", err)
	}

	return rescueFunding(
		db, signer, dbOp, chainOp, sweepScript,
		btcutil.Amount(c.FeeRate),
	)
}

func rescueFunding(db *channeldb.DB, signer *lnd.Signer, dbFundingPoint,
	chainPoint *wire.OutPoint, sweepPKScript []byte,
	feeRate btcutil.Amount) error {

	// First of all make sure the channel can be found in the DB.
	pendingChan, err := db.FetchChannel(*dbFundingPoint)
	if err != nil {
		return fmt.Errorf("error loading pending channel %s from DB: "+
			"%v", dbFundingPoint, err)
	}

	// Prepare the wire part of the PSBT.
	txIn := &wire.TxIn{
		PreviousOutPoint: *chainPoint,
		Sequence:         0,
	}
	txOut := &wire.TxOut{
		PkScript: sweepPKScript,
	}

	// Locate the output in the funding TX.
	utxo := pendingChan.FundingTxn.TxOut[dbFundingPoint.Index]

	// We should also be able to create the funding script from the two
	// multisig keys.
	localKey := pendingChan.LocalChanCfg.MultiSigKey.PubKey
	remoteKey := pendingChan.RemoteChanCfg.MultiSigKey.PubKey
	witnessScript, fundingTxOut, err := input.GenFundingPkScript(
		localKey.SerializeCompressed(), remoteKey.SerializeCompressed(),
		utxo.Value,
	)
	if err != nil {
		return fmt.Errorf("could not derive funding script: %v", err)
	}

	// Some last sanity check that we're working with the correct data.
	if !bytes.Equal(fundingTxOut.PkScript, utxo.PkScript) {
		return fmt.Errorf("funding output script does not match UTXO")
	}

	// Now the rest of the known data for the PSBT.
	pIn := psbt.PInput{
		WitnessUtxo:   utxo,
		WitnessScript: witnessScript,
		Unknowns: []*psbt.Unknown{{
			// We add the public key the other party needs to sign
			// with as a proprietary field so we can easily read it
			// out with the signrescuefunding command.
			Key:   PsbtKeyTypeOutputMissingSigPubkey,
			Value: remoteKey.SerializeCompressed(),
		}},
	}

	// Estimate the transaction weight so we can do the fee estimation.
	var estimator input.TxWeightEstimator
	estimator.AddWitnessInput(MultiSigWitnessSize)
	estimator.AddP2WKHOutput()
	feeRateKWeight := chainfee.SatPerKVByte(1000 * feeRate).FeePerKWeight()
	totalFee := feeRateKWeight.FeeForWeight(int64(estimator.Weight()))
	txOut.Value = utxo.Value - int64(totalFee)

	// Let's now create the PSBT as we have everything we need so far.
	wireTx := &wire.MsgTx{
		Version: 2,
		TxIn:    []*wire.TxIn{txIn},
		TxOut:   []*wire.TxOut{txOut},
	}
	packet, err := psbt.NewFromUnsignedTx(wireTx)
	if err != nil {
		return fmt.Errorf("error creating PSBT: %v", err)
	}
	packet.Inputs[0] = pIn

	// Now we add our partial signature.
	err = signer.AddPartialSignature(
		packet, pendingChan.LocalChanCfg.MultiSigKey, utxo,
		witnessScript, 0,
	)
	if err != nil {
		return fmt.Errorf("error adding partial signature: %v", err)
	}

	// We're done, we can now output the finished PSBT.
	base64, err := packet.B64Encode()
	if err != nil {
		return fmt.Errorf("error encoding PSBT: %v", err)
	}

	fmt.Printf("Partially signed transaction created. Send this to the "+
		"other peer \nand ask them to run the 'chantools "+
		"signrescuefunding' command: \n\n%s\n\n", base64)

	return nil
}
