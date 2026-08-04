package main

import (
	"errors"

	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/lnwallet"
)

// htlcScript builds the segwit v0 HTLC script and describes the required spend
// path for a channel HTLC on a specific commitment side.
func htlcScript(chanType channeldb.ChannelType, whoseCommit lntypes.ChannelParty,
	htlc channeldb.HTLC, keyRing *lnwallet.CommitmentKeyRing) (
	[]byte, string, string, bool, error) {

	confirmedSpend := chanType.HasAnchors()
	switch {
	case htlc.Incoming && whoseCommit.IsLocal():
		script, err := input.ReceiverHTLCScript(
			htlc.RefundTimeout, keyRing.RemoteHtlcKey,
			keyRing.LocalHtlcKey, keyRing.RevocationKey,
			htlc.RHash[:], confirmedSpend,
		)
		return script, "incoming/accepted_by_us", "success", false, err

	case htlc.Incoming && whoseCommit.IsRemote():
		script, err := input.SenderHTLCScript(
			keyRing.RemoteHtlcKey, keyRing.LocalHtlcKey,
			keyRing.RevocationKey, htlc.RHash[:], confirmedSpend,
		)
		return script, "incoming/accepted_by_us", "success", false, err

	case !htlc.Incoming && whoseCommit.IsLocal():
		script, err := input.SenderHTLCScript(
			keyRing.LocalHtlcKey, keyRing.RemoteHtlcKey,
			keyRing.RevocationKey, htlc.RHash[:], confirmedSpend,
		)
		return script, "outgoing/offered_by_us", "timeout", false, err

	case !htlc.Incoming && whoseCommit.IsRemote():
		script, err := input.ReceiverHTLCScript(
			htlc.RefundTimeout, keyRing.LocalHtlcKey,
			keyRing.RemoteHtlcKey, keyRing.RevocationKey,
			htlc.RHash[:], confirmedSpend,
		)
		return script, "outgoing/offered_by_us", "timeout", true, err
	}

	return nil, "", "", false, errors.New("unknown HTLC direction")
}
