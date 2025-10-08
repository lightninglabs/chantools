package itest

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

func runSweepRemoteClosedLnd(t *testing.T) {
	sweepAddr := randTaprootAddr(t)
	txHex := getSweepRemoteClosed(
		t, "charlie", tempDir, localElectrsAddr, sweepAddr,
	)

	txBytes, err := hex.DecodeString(txHex)
	require.NoError(t, err)

	var tx wire.MsgTx
	err = tx.Deserialize(bytes.NewReader(txBytes))
	require.NoError(t, err)

	backend := connectBitcoind(t)
	txHash, err := backend.SendRawTransaction(&tx, false)
	require.NoError(t, err)
	t.Logf("Sweep transaction sent: %v", txHash.String())
}

func runSweepRemoteClosedCln(t *testing.T) {
	aliceIdentity := getNodeIdentityKey(t, "alice")
	sweepAddr := randTaprootAddr(t)
	txHex := getSweepRemoteClosedCln(
		t, "rusty", tempDir, localElectrsAddr, aliceIdentity, sweepAddr,
	)

	txBytes, err := hex.DecodeString(txHex)
	require.NoError(t, err)

	var tx wire.MsgTx
	err = tx.Deserialize(bytes.NewReader(txBytes))
	require.NoError(t, err)

	backend := connectBitcoind(t)
	txHash, err := backend.SendRawTransaction(&tx, false)
	require.NoError(t, err)
	t.Logf("Sweep transaction sent: %v", txHash.String())
}
