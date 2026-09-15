package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightninglabs/chantools/btc"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/lnwallet"
	"github.com/lightningnetwork/lnd/lnwallet/chainfee"
)

// sweepMatchedHtlcs signs and optionally publishes the sweep transaction for
// all matched direct-timeout HTLCs.
func sweepMatchedHtlcs(extendedKey *hdkeychain.ExtendedKey,
	api *btc.ExplorerAPI, matches []*sweepHtlcMatch, sweepAddr string,
	feeRate uint32, publish bool) error {

	if len(matches) == 0 {
		return errors.New("no HTLC matches to sweep")
	}

	for _, match := range matches {
		log.Infof("Matched %v: channel=%v (%s), commitment=%s, "+
			"commit_point_source=%s, direction=%s, spend_path=%s, "+
			"amount=%d, expiry=%d, payment_hash=%x",
			match.target.outpoint, match.channel.FundingOutpoint,
			match.channelSource, match.commitmentName,
			match.commitPointSrc, match.direction, match.spendPath,
			match.target.value, match.htlc.RefundTimeout,
			match.htlc.RHash)

		if !match.supportedDirect {
			return fmt.Errorf("matched HTLC %v requires unsupported spend "+
				"path: commitment=%s direction=%s spend_path=%s",
				match.target.outpoint, match.commitmentName,
				match.direction, match.spendPath)
		}
	}

	var estimator input.TxWeightEstimator
	sweepScript, err := lnd.PrepareWalletAddress(
		sweepAddr, chainParams, &estimator, extendedKey, "sweep",
	)
	if err != nil {
		return err
	}

	signer := &lnd.Signer{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}
	sweepTx := wire.NewMsgTx(2)
	prevOutFetcher := txscript.NewMultiPrevOutFetcher(nil)
	signDescs := make([]*input.SignDescriptor, 0, len(matches))
	totalOutputValue := int64(0)
	maxLockTime := uint32(0)

	for _, match := range matches {
		prevOut := &wire.TxOut{
			PkScript: match.pkScript,
			Value:    match.target.value,
		}
		prevOutFetcher.AddPrevOut(match.target.outpoint, prevOut)

		sequence := lnwallet.HtlcSecondLevelInputSequence(
			match.channel.ChanType,
		)
		sweepTx.TxIn = append(sweepTx.TxIn, &wire.TxIn{
			PreviousOutPoint: match.target.outpoint,
			Sequence:         sequence,
		})

		signDescs = append(signDescs, &input.SignDescriptor{
			KeyDesc:           match.channel.LocalChanCfg.HtlcBasePoint,
			SingleTweak:       match.keyRing.LocalHtlcKeyTweak,
			WitnessScript:     match.witnessScript,
			Output:            prevOut,
			HashType:          txscript.SigHashAll,
			PrevOutputFetcher: prevOutFetcher,
		})

		if match.channel.ChanType.HasAnchors() {
			estimator.AddWitnessInput(
				input.AcceptedHtlcTimeoutWitnessSizeConfirmed,
			)
		} else {
			estimator.AddWitnessInput(input.AcceptedHtlcTimeoutWitnessSize)
		}

		totalOutputValue += match.target.value
		if match.htlc.RefundTimeout > maxLockTime {
			maxLockTime = match.htlc.RefundTimeout
		}
	}

	sweepTx.LockTime = maxLockTime
	feeRateKWeight := chainfee.SatPerKVByte(1000 * feeRate).FeePerKWeight()
	totalFee := feeRateKWeight.FeeForWeight(estimator.Weight())
	if int64(totalFee) >= totalOutputValue {
		return fmt.Errorf("fee %d exceeds total output value %d",
			totalFee, totalOutputValue)
	}

	log.Infof("Fee %d sats of %d total amount (estimated weight %d)",
		totalFee, totalOutputValue, estimator.Weight())

	sweepTx.TxOut = []*wire.TxOut{{
		Value:    totalOutputValue - int64(totalFee),
		PkScript: sweepScript,
	}}

	sigHashes := txscript.NewTxSigHashes(sweepTx, prevOutFetcher)
	for idx, desc := range signDescs {
		desc.SigHashes = sigHashes
		desc.InputIndex = idx
		witness, err := input.ReceiverHtlcSpendTimeout(
			signer, desc, sweepTx, -1,
		)
		if err != nil {
			return err
		}
		sweepTx.TxIn[idx].Witness = witness
	}

	var buf bytes.Buffer
	if err := sweepTx.Serialize(&buf); err != nil {
		return err
	}

	if publish {
		response, err := api.PublishTx(hex.EncodeToString(buf.Bytes()))
		if err != nil {
			return err
		}
		log.Infof("Published TX %s, response: %s",
			sweepTx.TxHash().String(), response)
	}

	log.Infof("Transaction: %x", buf.Bytes())
	return nil
}
