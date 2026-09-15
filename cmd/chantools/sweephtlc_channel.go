package main

import (
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightningnetwork/lnd/channeldb"
)

// findSweepHtlcMatches loads the channels for the targets and matches each one.
func findSweepHtlcMatches(chanDB *channeldb.ChannelStateDB,
	targets []*sweepHtlcTarget, commitPointOverride *btcec.PublicKey) (
	[]*sweepHtlcMatch, error) {

	matches := make([]*sweepHtlcMatch, 0, len(targets))
	for _, target := range targets {
		channel, source, err := fetchSweepHtlcChannel(
			chanDB, target.fundingPoint,
		)
		if err != nil {
			return nil, err
		}

		candidateMatches, err := matchTargetHtlc(
			channel, source, target, commitPointOverride,
		)
		if err != nil {
			return nil, err
		}

		switch len(candidateMatches) {
		case 0:
			return nil, fmt.Errorf("no HTLC in channel %v matched %v",
				target.fundingPoint, target.outpoint)

		case 1:
			matches = append(matches, candidateMatches[0])

		default:
			return nil, fmt.Errorf("multiple HTLC candidates matched %v; "+
				"refusing to guess", target.outpoint)
		}
	}

	return matches, nil
}

// fetchSweepHtlcChannel finds a channel in either the open or historical bucket.
func fetchSweepHtlcChannel(chanDB *channeldb.ChannelStateDB,
	chanPoint wire.OutPoint) (*channeldb.OpenChannel, string, error) {

	channel, err := chanDB.FetchChannel(chanPoint)
	if err == nil {
		return channel, "open", nil
	}

	historical, histErr := chanDB.FetchHistoricalChannel(&chanPoint)
	if histErr == nil {
		return historical, "historical", nil
	}

	return nil, "", fmt.Errorf("channel %v not found: open lookup: %v, "+
		"historical lookup: %v", chanPoint, err, histErr)
}
