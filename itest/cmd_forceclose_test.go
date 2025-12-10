package itest

import (
	"fmt"
	"testing"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/stretchr/testify/require"
)

func runForceClose(t *testing.T) {
	aliceChannels := readChannelsJSON(t, "alice")
	niftyIdentity := getNodeIdentityKeyCln(t, "nifty")

	var aliceNiftyChannel *lnrpc.Channel
	for _, c := range aliceChannels {
		if c.RemotePubkey == niftyIdentity {
			aliceNiftyChannel = c
		}
	}
	require.NotNil(t, aliceNiftyChannel, "alice-nifty channel not found")

	channelDB := fmt.Sprintf(channelDBFilePattern, "alice")
	txHex, _ := getForceClose(
		t, "alice", tempDir, channelDB, aliceNiftyChannel.ChannelPoint,
	)

	backend := connectBitcoind(t)
	publishTx(t, txHex, backend)
}
