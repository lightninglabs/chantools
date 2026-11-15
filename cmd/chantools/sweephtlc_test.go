package main

import (
	"bytes"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwallet"
	"github.com/lightningnetwork/lnd/lnwire"
	"github.com/lightningnetwork/lnd/shachain"
	"github.com/stretchr/testify/require"
)

// testSweepHtlcPubKey returns a deterministic public key for tests.
func testSweepHtlcPubKey(seed byte) *btcec.PublicKey {
	_, pubKey := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{seed}, 32))
	return pubKey
}

// testSweepHtlcConfig returns a deterministic channel config for tests.
func testSweepHtlcConfig(seed byte) channeldb.ChannelConfig {
	return channeldb.ChannelConfig{
		MultiSigKey: keychain.KeyDescriptor{
			PubKey: testSweepHtlcPubKey(seed),
		},
		RevocationBasePoint: keychain.KeyDescriptor{
			PubKey: testSweepHtlcPubKey(seed + 1),
		},
		PaymentBasePoint: keychain.KeyDescriptor{
			PubKey: testSweepHtlcPubKey(seed + 2),
		},
		DelayBasePoint: keychain.KeyDescriptor{
			PubKey: testSweepHtlcPubKey(seed + 3),
		},
		HtlcBasePoint: keychain.KeyDescriptor{
			KeyLocator: keychain.KeyLocator{
				Family: keychain.KeyFamilyHtlcBase,
				Index:  uint32(seed),
			},
			PubKey: testSweepHtlcPubKey(seed + 4),
		},
	}
}

// testSweepHtlcChannel returns a minimal anchor channel for matching tests.
func testSweepHtlcChannel() *channeldb.OpenChannel {
	var revRoot chainhash.Hash
	copy(revRoot[:], bytes.Repeat([]byte{3}, 32))

	return &channeldb.OpenChannel{
		ChanType: channeldb.SingleFunderTweaklessBit |
			channeldb.AnchorOutputsBit |
			channeldb.ZeroHtlcTxFeeBit,
		FundingOutpoint:    wire.OutPoint{Index: 1},
		LocalChanCfg:       testSweepHtlcConfig(10),
		RemoteChanCfg:      testSweepHtlcConfig(50),
		RevocationProducer: shachain.NewRevocationProducer(revRoot),
	}
}

// testSweepHtlc returns a deterministic outgoing HTLC for matching tests.
func testSweepHtlc() channeldb.HTLC {
	var rHash [32]byte
	copy(rHash[:], bytes.Repeat([]byte{9}, 32))

	return channeldb.HTLC{
		RHash:         rHash,
		Amt:           lnwire.NewMSatFromSatoshis(12345),
		RefundTimeout: 900000,
		OutputIndex:   3,
		Incoming:      false,
		HtlcIndex:     11,
	}
}

// TestMatchSingleRemoteOfferedHtlc verifies the supported direct timeout match.
func TestMatchSingleRemoteOfferedHtlc(t *testing.T) {
	channel := testSweepHtlcChannel()
	htlc := testSweepHtlc()
	_, commitPoint := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{1}, 32))

	keyRing := lnwallet.DeriveCommitmentKeys(
		commitPoint, lntypes.Remote, channel.ChanType,
		&channel.LocalChanCfg, &channel.RemoteChanCfg,
	)
	witnessScript, _, _, supported, err := htlcScript(
		channel.ChanType, lntypes.Remote, htlc, keyRing,
	)
	require.NoError(t, err)
	require.True(t, supported)

	pkScript, err := input.WitnessScriptHash(witnessScript)
	require.NoError(t, err)

	target := &sweepHtlcTarget{
		outpoint: wire.OutPoint{Index: 3},
		value:    int64(htlc.Amt.ToSatoshis()),
		pkScript: pkScript,
	}
	candidate := sweepHtlcCommitCandidate{
		name:       "remote_commitment",
		side:       lntypes.Remote,
		commitment: channeldb.ChannelCommitment{Htlcs: []channeldb.HTLC{htlc}},
	}
	cp := sweepHtlcCommitPoint{point: commitPoint, source: "test"}

	match, matched, err := matchSingleHtlc(
		channel, "test", target, candidate, cp, keyRing, htlc,
	)
	require.NoError(t, err)
	require.True(t, matched)
	require.NotNil(t, match)
	require.True(t, match.supportedDirect)
	require.Equal(t, "remote_commitment", match.commitmentName)
	require.Equal(t, "outgoing/offered_by_us", match.direction)
	require.Equal(t, "timeout", match.spendPath)
}

// TestMatchSingleHtlcWrongCommitPoint verifies that script mismatches are
// reported as no match.
func TestMatchSingleHtlcWrongCommitPoint(t *testing.T) {
	channel := testSweepHtlcChannel()
	htlc := testSweepHtlc()
	_, correctCommitPoint := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{1}, 32))
	_, wrongCommitPoint := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{2}, 32))

	correctKeyRing := lnwallet.DeriveCommitmentKeys(
		correctCommitPoint, lntypes.Remote, channel.ChanType,
		&channel.LocalChanCfg, &channel.RemoteChanCfg,
	)
	witnessScript, direction, spendPath, supported, err := htlcScript(
		channel.ChanType, lntypes.Remote, htlc, correctKeyRing,
	)
	require.NoError(t, err)
	require.Equal(t, "outgoing/offered_by_us", direction)
	require.Equal(t, "timeout", spendPath)
	require.True(t, supported)
	pkScript, err := input.WitnessScriptHash(witnessScript)
	require.NoError(t, err)

	wrongKeyRing := lnwallet.DeriveCommitmentKeys(
		wrongCommitPoint, lntypes.Remote, channel.ChanType,
		&channel.LocalChanCfg, &channel.RemoteChanCfg,
	)
	target := &sweepHtlcTarget{
		outpoint: wire.OutPoint{Index: 3},
		value:    int64(htlc.Amt.ToSatoshis()),
		pkScript: pkScript,
	}
	candidate := sweepHtlcCommitCandidate{
		name:       "remote_commitment",
		side:       lntypes.Remote,
		commitment: channeldb.ChannelCommitment{Htlcs: []channeldb.HTLC{htlc}},
	}
	cp := sweepHtlcCommitPoint{point: wrongCommitPoint, source: "test"}

	match, matched, err := matchSingleHtlc(
		channel, "test", target, candidate, cp, wrongKeyRing, htlc,
	)
	require.NoError(t, err)
	require.False(t, matched)
	require.Nil(t, match)
}

// TestMatchTargetHtlcDlpCommitPointOverride verifies the DLP case where the
// HTLC metadata exists locally but the matching commitment point must be
// supplied by the peer's DLP message.
func TestMatchTargetHtlcDlpCommitPointOverride(t *testing.T) {
	channel := testSweepHtlcChannel()
	htlc := testSweepHtlc()
	_, staleCommitPoint := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{1}, 32))
	_, dlpCommitPoint := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{2}, 32))

	channel.RemoteCurrentRevocation = staleCommitPoint
	channel.RemoteCommitment = channeldb.ChannelCommitment{
		Htlcs: []channeldb.HTLC{htlc},
	}

	dlpKeyRing := lnwallet.DeriveCommitmentKeys(
		dlpCommitPoint, lntypes.Remote, channel.ChanType,
		&channel.LocalChanCfg, &channel.RemoteChanCfg,
	)
	witnessScript, direction, spendPath, supported, err := htlcScript(
		channel.ChanType, lntypes.Remote, htlc, dlpKeyRing,
	)
	require.NoError(t, err)
	require.Equal(t, "outgoing/offered_by_us", direction)
	require.Equal(t, "timeout", spendPath)
	require.True(t, supported)

	pkScript, err := input.WitnessScriptHash(witnessScript)
	require.NoError(t, err)
	target := &sweepHtlcTarget{
		outpoint: wire.OutPoint{Index: 3},
		value:    int64(htlc.Amt.ToSatoshis()),
		pkScript: pkScript,
	}

	matches, err := matchTargetHtlc(channel, "test", target, nil)
	require.NoError(t, err)
	require.Empty(t, matches)

	matches, err = matchTargetHtlc(channel, "test", target, dlpCommitPoint)
	require.NoError(t, err)
	require.Len(t, matches, 1)
	require.Equal(t, "remote_commitment", matches[0].commitmentName)
	require.Equal(t, "override", matches[0].commitPointSrc)
	require.True(t, matches[0].supportedDirect)
}

// TestMatchSingleLocalOfferedHtlcUnsupported verifies local commitment matches
// are detected but rejected by the signing path.
func TestMatchSingleLocalOfferedHtlcUnsupported(t *testing.T) {
	channel := testSweepHtlcChannel()
	htlc := testSweepHtlc()
	_, commitPoint := btcec.PrivKeyFromBytes(bytes.Repeat([]byte{1}, 32))

	keyRing := lnwallet.DeriveCommitmentKeys(
		commitPoint, lntypes.Local, channel.ChanType,
		&channel.LocalChanCfg, &channel.RemoteChanCfg,
	)
	witnessScript, _, _, supported, err := htlcScript(
		channel.ChanType, lntypes.Local, htlc, keyRing,
	)
	require.NoError(t, err)
	require.False(t, supported)

	pkScript, err := input.WitnessScriptHash(witnessScript)
	require.NoError(t, err)
	target := &sweepHtlcTarget{
		outpoint: wire.OutPoint{Index: 3},
		value:    int64(htlc.Amt.ToSatoshis()),
		pkScript: pkScript,
	}
	candidate := sweepHtlcCommitCandidate{
		name:       "local_commitment",
		side:       lntypes.Local,
		commitment: channeldb.ChannelCommitment{Htlcs: []channeldb.HTLC{htlc}},
	}
	cp := sweepHtlcCommitPoint{point: commitPoint, source: "test"}

	match, matched, err := matchSingleHtlc(
		channel, "test", target, candidate, cp, keyRing, htlc,
	)
	require.NoError(t, err)
	require.True(t, matched)
	require.NotNil(t, match)
	require.False(t, match.supportedDirect)
	require.Equal(t, "local_commitment", match.commitmentName)
}
