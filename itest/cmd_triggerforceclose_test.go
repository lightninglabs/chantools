package itest

import (
	"fmt"
	"testing"

	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/stretchr/testify/require"
)

func runTriggerForceCloseLnd(t *testing.T) {
	daveChannels := readChannelsJSON(t, "dave")
	charlieIdentity := getNodeIdentityKey(t, "charlie")
	daveIdentity := readNodeIdentityFromFile(t, "dave")
	daveURI := fmt.Sprintf(nodeURIPattern, daveIdentity, localDaveAddr)

	var charlieDaveChannel *lnrpc.Channel
	for _, c := range daveChannels {
		if c.RemotePubkey == charlieIdentity {
			charlieDaveChannel = c
		}
	}
	require.NotNil(t, charlieDaveChannel, "charlie-dave channel not found")

	txid := getTriggerForceClose(
		t, "charlie", tempDir, localElectrsAddr, daveURI,
		charlieDaveChannel.ChannelPoint,
	)

	t.Logf("Force close TX found: %v", txid)
}

func runTriggerForceCloseCln(t *testing.T) {
	aliceChannels := readChannelsJSON(t, "alice")
	snykeIdentity := getNodeIdentityKeyCln(t, "snyke")
	snykeURI := fmt.Sprintf(nodeURIPattern, snykeIdentity, localSnykeAddr)

	var aliceSnykeChannel *lnrpc.Channel
	for _, c := range aliceChannels {
		if c.RemotePubkey == snykeIdentity {
			aliceSnykeChannel = c
		}
	}
	require.NotNil(t, aliceSnykeChannel, "alice-snyke channel not found")

	txid := getTriggerForceClose(
		t, "alice", tempDir, localElectrsAddr, snykeURI,
		aliceSnykeChannel.ChannelPoint,
	)

	t.Logf("Force close TX found: %v", txid)
}
