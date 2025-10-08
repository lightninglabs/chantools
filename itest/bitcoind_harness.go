package itest

import (
	"strconv"
	"strings"
	"testing"

	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/stretchr/testify/require"
)

func connectBitcoind(t *testing.T) *rpcclient.Client {
	t.Helper()

	rpcCfg := rpcclient.ConnConfig{
		Host:                 "127.0.0.1:18443",
		User:                 "lightning",
		Pass:                 "lightning",
		DisableConnectOnNew:  true,
		DisableAutoReconnect: false,
		DisableTLS:           true,
		HTTPPostMode:         true,
	}

	client, err := rpcclient.New(&rpcCfg, nil)
	require.NoError(t, err)

	return client
}

func addressOfOutpoint(t *testing.T, client *rpcclient.Client,
	op string) string {

	t.Helper()

	channelOp, err := wire.NewOutPointFromString(op)
	require.NoError(t, err)

	channelTx, err := client.GetRawTransaction(&channelOp.Hash)
	require.NoError(t, err)

	channelScript := channelTx.MsgTx().TxOut[channelOp.Index].PkScript
	_, addrs, _, err := txscript.ExtractPkScriptAddrs(
		channelScript, &testParams,
	)
	require.NoError(t, err)
	require.Len(t, addrs, 1)

	return addrs[0].EncodeAddress()
}

func addrAndOpFromShortChannelID(t *testing.T, client *rpcclient.Client,
	shortChanID string) (string, wire.OutPoint) {

	t.Helper()

	parts := strings.Split(shortChanID, "x")
	require.Len(t, parts, 3)

	blockHeight, err := strconv.Atoi(parts[0])
	require.NoError(t, err)
	txIndex, err := strconv.Atoi(parts[1])
	require.NoError(t, err)
	outputIndex, err := strconv.Atoi(parts[2])
	require.NoError(t, err)

	blockHash, err := client.GetBlockHash(int64(blockHeight))
	require.NoError(t, err)

	block, err := client.GetBlock(blockHash)
	require.NoError(t, err)
	require.Greater(t, len(block.Transactions), outputIndex)

	tx := block.Transactions[txIndex]
	op := wire.OutPoint{
		Hash:  tx.TxHash(),
		Index: uint32(outputIndex),
	}

	return addressOfOutpoint(t, client, op.String()), op
}
