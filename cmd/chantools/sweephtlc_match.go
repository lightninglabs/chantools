package main

import (
	"bytes"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwallet"
)

// sweepHtlcMatch contains the channel HTLC metadata matched to a target output.
type sweepHtlcMatch struct {
	// target is the on-chain HTLC output being swept.
	target *sweepHtlcTarget

	// channel is the channel.db record that owns the target output.
	channel *channeldb.OpenChannel

	// channelSource identifies whether the channel was open or historical.
	channelSource string

	// commitmentName identifies the commitment candidate that matched.
	commitmentName string

	// commitmentSide identifies whose commitment transaction created the output.
	commitmentSide lntypes.ChannelParty

	// commitPoint is the commitment point used to derive the HTLC script.
	commitPoint *btcec.PublicKey

	// commitPointSrc identifies where commitPoint came from.
	commitPointSrc string

	// htlc is the channel.db HTLC entry that matched the target output.
	htlc channeldb.HTLC

	// keyRing is the commitment key ring for the matched commitment point.
	keyRing *lnwallet.CommitmentKeyRing

	// witnessScript is the HTLC witness script that matched the output.
	witnessScript []byte

	// pkScript is the witness script hash output script.
	pkScript []byte

	// direction describes whether the HTLC was incoming or outgoing locally.
	direction string

	// spendPath describes the HTLC script branch needed to spend the output.
	spendPath string

	// supportedDirect marks whether sweephtlc can directly spend this match.
	supportedDirect bool
}

// matchTargetHtlc reconstructs candidate HTLC scripts and compares them to the
// target output.
func matchTargetHtlc(channel *channeldb.OpenChannel, channelSource string,
	target *sweepHtlcTarget, commitPointOverride *btcec.PublicKey) (
	[]*sweepHtlcMatch, error) {

	if channel.ChanType.IsTaproot() {
		return nil, fmt.Errorf("channel %v is taproot; sweephtlc v1 only "+
			"supports segwit v0 HTLC outputs", channel.FundingOutpoint)
	}

	candidates := sweepHtlcCommitCandidates(channel, commitPointOverride)
	var matches []*sweepHtlcMatch
	for _, candidate := range candidates {
		for _, cp := range candidate.commitPoints {
			if cp.point == nil {
				continue
			}

			keyRing := lnwallet.DeriveCommitmentKeys(
				cp.point, candidate.side, channel.ChanType,
				&channel.LocalChanCfg, &channel.RemoteChanCfg,
			)

			for _, htlc := range candidate.commitment.Htlcs {
				match, matched, err := matchSingleHtlc(
					channel, channelSource, target, candidate, cp,
					keyRing, htlc,
				)
				if err != nil {
					return nil, err
				}
				if matched {
					matches = append(matches, match)
				}
			}
		}
	}

	return matches, nil
}

// matchSingleHtlc checks whether one channel HTLC matches one target output.
func matchSingleHtlc(channel *channeldb.OpenChannel, channelSource string,
	target *sweepHtlcTarget, candidate sweepHtlcCommitCandidate,
	cp sweepHtlcCommitPoint, keyRing *lnwallet.CommitmentKeyRing,
	htlc channeldb.HTLC) (*sweepHtlcMatch, bool, error) {

	if htlc.OutputIndex < 0 {
		return nil, false, nil
	}
	if uint32(htlc.OutputIndex) != target.outpoint.Index {
		return nil, false, nil
	}
	if int64(htlc.Amt.ToSatoshis()) != target.value {
		return nil, false, nil
	}

	witnessScript, direction, spendPath, supportedDirect, err := htlcScript(
		channel.ChanType, candidate.side, htlc, keyRing,
	)
	if err != nil {
		return nil, false, err
	}

	pkScript, err := input.WitnessScriptHash(witnessScript)
	if err != nil {
		return nil, false, err
	}
	if !bytes.Equal(pkScript, target.pkScript) {
		return nil, false, nil
	}

	return &sweepHtlcMatch{
		target:          target,
		channel:         channel,
		channelSource:   channelSource,
		commitmentName:  candidate.name,
		commitmentSide:  candidate.side,
		commitPoint:     cp.point,
		commitPointSrc:  cp.source,
		htlc:            htlc,
		keyRing:         keyRing,
		witnessScript:   witnessScript,
		pkScript:        pkScript,
		direction:       direction,
		spendPath:       spendPath,
		supportedDirect: supportedDirect,
	}, true, nil
}
