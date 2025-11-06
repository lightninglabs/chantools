package itest

import (
	"fmt"
	"testing"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/stretchr/testify/require"
)

func runScbForceClose(t *testing.T) {
	charlieChannels := readChannelsJSON(t, "charlie")
	snykeIdentity := getNodeIdentityKeyCln(t, "snyke")

	var charlieSnykeChannel *lnrpc.Channel
	for _, c := range charlieChannels {
		if c.RemotePubkey == snykeIdentity {
			charlieSnykeChannel = c
		}
	}
	require.NotNil(
		t, charlieSnykeChannel, "charlie-snyke channel not found",
	)

	scbFile := fmt.Sprintf(scbFilePattern, "charlie")
	txHex, fullOutput := getScbForceClose(
		t, "charlie", tempDir, scbFile,
		charlieSnykeChannel.ChannelPoint,
	)

	// Outputs on a force-close transaction are always ordered by amount.
	require.Contains(
		t, fullOutput, "Possible anchor: idx=0 amount=330 sat",
	)
	require.Contains(
		t, fullOutput, "Possible anchor: idx=1 amount=330 sat",
	)
	require.Contains(t, fullOutput, "Output to_remote: idx=2 amount=")
	require.Contains(t, fullOutput, "Possible to_local/htlc: idx=3 amount=")

	backend := connectBitcoind(t)
	publishTx(t, txHex, backend)
}
