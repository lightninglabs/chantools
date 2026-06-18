package main

import (
	"encoding/hex"
	"errors"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/lntypes"
)

// sweepHtlcCommitPoint pairs a commitment point with its source label.
type sweepHtlcCommitPoint struct {
	// point is the commitment point used for key derivation.
	point *btcec.PublicKey

	// source identifies where the commitment point was loaded from.
	source string
}

// sweepHtlcCommitCandidate describes one commitment state to test for matches.
type sweepHtlcCommitCandidate struct {
	// name is a human readable label for the commitment candidate.
	name string

	// side identifies whose commitment transaction this candidate represents.
	side lntypes.ChannelParty

	// commitment contains the HTLC set to test against the target output.
	commitment channeldb.ChannelCommitment

	// commitPoints are the commitment points to try for this candidate.
	commitPoints []sweepHtlcCommitPoint
}

// sweepHtlcCommitCandidates returns the local, remote, and pending remote
// commitment candidates that can be reconstructed from channel.db.
func sweepHtlcCommitCandidates(channel *channeldb.OpenChannel,
	commitPointOverride *btcec.PublicKey) []sweepHtlcCommitCandidate {

	localPoints := make([]sweepHtlcCommitPoint, 0, 2)
	remotePoints := make([]sweepHtlcCommitPoint, 0, 3)

	if commitPointOverride != nil {
		localPoints = append(localPoints, sweepHtlcCommitPoint{
			point:  commitPointOverride,
			source: "override",
		})
		remotePoints = append(remotePoints, sweepHtlcCommitPoint{
			point:  commitPointOverride,
			source: "override",
		})
	}

	localCommitPoint, err := deriveLocalCommitPoint(channel)
	if err == nil {
		localPoints = append(localPoints, sweepHtlcCommitPoint{
			point:  localCommitPoint,
			source: "revocation_producer",
		})
	} else {
		log.Warnf("Unable to derive local commit point for %v: %v",
			channel.FundingOutpoint, err)
	}

	remotePoints = appendPubKeyCandidate(
		remotePoints, channel.RemoteCurrentRevocation,
		"remote_current_revocation",
	)
	remotePoints = appendPubKeyCandidate(
		remotePoints, channel.RemoteNextRevocation, "remote_next_revocation",
	)

	candidates := []sweepHtlcCommitCandidate{{
		name:         "local_commitment",
		side:         lntypes.Local,
		commitment:   channel.LocalCommitment,
		commitPoints: dedupeCommitPoints(localPoints),
	}, {
		name:         "remote_commitment",
		side:         lntypes.Remote,
		commitment:   channel.RemoteCommitment,
		commitPoints: dedupeCommitPoints(remotePoints),
	}}

	if channel.Db == nil {
		return candidates
	}

	if tip, err := channel.RemoteCommitChainTip(); err == nil {
		pendingRemotePoints := make([]sweepHtlcCommitPoint, 0, 2)
		if commitPointOverride != nil {
			pendingRemotePoints = append(
				pendingRemotePoints, sweepHtlcCommitPoint{
					point:  commitPointOverride,
					source: "override",
				},
			)
		}
		pendingRemotePoints = appendPubKeyCandidate(
			pendingRemotePoints, channel.RemoteNextRevocation,
			"remote_next_revocation",
		)

		candidates = append(candidates, sweepHtlcCommitCandidate{
			name:         "pending_remote_commitment",
			side:         lntypes.Remote,
			commitment:   tip.Commitment,
			commitPoints: dedupeCommitPoints(pendingRemotePoints),
		})
	} else if !errors.Is(err, channeldb.ErrNoPendingCommit) {
		log.Warnf("Unable to fetch pending remote commitment for %v: %v",
			channel.FundingOutpoint, err)
	}

	return candidates
}

// deriveLocalCommitPoint derives the local commitment point from the local
// revocation producer.
func deriveLocalCommitPoint(channel *channeldb.OpenChannel) (*btcec.PublicKey, error) {
	if channel.RevocationProducer == nil {
		return nil, errors.New("missing revocation producer")
	}

	rev, err := channel.RevocationProducer.AtIndex(
		channel.LocalCommitment.CommitHeight,
	)
	if err != nil {
		return nil, err
	}

	return input.ComputeCommitmentPoint(rev[:]), nil
}

// appendPubKeyCandidate adds a non-nil public key to the candidate list.
func appendPubKeyCandidate(candidates []sweepHtlcCommitPoint,
	pubKey *btcec.PublicKey, source string) []sweepHtlcCommitPoint {

	if pubKey == nil {
		return candidates
	}

	return append(candidates, sweepHtlcCommitPoint{
		point:  pubKey,
		source: source,
	})
}

// dedupeCommitPoints removes duplicate commitment points while preserving order.
func dedupeCommitPoints(points []sweepHtlcCommitPoint) []sweepHtlcCommitPoint {
	seen := make(map[string]struct{})
	deduped := make([]sweepHtlcCommitPoint, 0, len(points))
	for _, point := range points {
		if point.point == nil {
			continue
		}

		key := hex.EncodeToString(point.point.SerializeCompressed())
		if _, ok := seen[key]; ok {
			continue
		}

		seen[key] = struct{}{}
		deduped = append(deduped, point)
	}

	return deduped
}
