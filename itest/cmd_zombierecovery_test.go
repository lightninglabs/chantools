package itest

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strconv"
	"testing"

	"github.com/btcsuite/btcd/rpcclient"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/stretchr/testify/require"
)

const (
	tempDir = "./docker/node-data/chantools"
)

type zombieNode struct {
	PubKey       string   `json:"identity_pubkey"`
	Contact      string   `json:"contact"`
	PayoutAddr   string   `json:"payout_addr,omitempty"`
	MultisigKeys []string `json:"multisig_keys,omitempty"`
}

type zombieChannel struct {
	ChannelID string `json:"short_channel_id"`
	ChanPoint string `json:"chan_point"`
	Address   string `json:"address"`
	Capacity  int64  `json:"capacity"`
}

type zombieMatch struct {
	Node1    *zombieNode      `json:"node1"`
	Node2    *zombieNode      `json:"node2"`
	Channels []*zombieChannel `json:"channels"`
}

func makeMatchFile(t *testing.T, node1Name, node2Name, node1Identity,
	node2Identity, channelPoint, channelID, address string,
	capacity int64) string {

	match := &zombieMatch{
		Node1: &zombieNode{
			PubKey:  node1Identity,
			Contact: node1Name,
		},
		Node2: &zombieNode{
			PubKey:  node2Identity,
			Contact: node2Name,
		},
		Channels: []*zombieChannel{{
			ChannelID: channelID,
			ChanPoint: channelPoint,
			Address:   address,
			Capacity:  capacity,
		}},
	}

	matchFile := fmt.Sprintf("%s/%s-%s-match.json", tempDir, node1Name,
		node2Name)
	err := writeJSONFile(matchFile, match)
	require.NoError(t, err)

	return matchFile
}

func publishTx(t *testing.T, txHex string, backend *rpcclient.Client) {
	txBytes, err := hex.DecodeString(txHex)
	require.NoError(t, err)

	var tx wire.MsgTx
	err = tx.Deserialize(bytes.NewReader(txBytes))
	require.NoError(t, err)

	txHash, err := backend.SendRawTransaction(&tx, false)
	require.NoError(t, err)
	t.Logf("Closing transaction sent: %v", txHash.String())
}

func runZombieRecoveryLndLnd(t *testing.T) {
	aliceChannels := readChannelsJSON(t, "alice")
	aliceIdentity := getNodeIdentityKey(t, "alice")
	bobIdentity := getNodeIdentityKey(t, "bob")
	aliceSweepAddr := randTaprootAddr(t)
	bobSweepAddr := randTaprootAddr(t)

	var aliceBobChannel *lnrpc.Channel
	for _, c := range aliceChannels {
		if c.RemotePubkey == bobIdentity {
			aliceBobChannel = c
		}
	}
	require.NotNil(t, aliceBobChannel, "alice-bob channel not found")

	backend := connectBitcoind(t)
	addr := addressOfOutpoint(t, backend, aliceBobChannel.ChannelPoint)
	matchFile := makeMatchFile(
		t, "alice", "bob", aliceIdentity, bobIdentity,
		aliceBobChannel.ChannelPoint,
		strconv.FormatUint(aliceBobChannel.ChanId, 10), addr,
		aliceBobChannel.Capacity,
	)

	resultsFileAlice := getZombiePreparedKeys(
		t, "alice", tempDir, matchFile, aliceSweepAddr,
	)
	resultsFileBob := getZombiePreparedKeys(
		t, "bob", tempDir, matchFile, bobSweepAddr,
	)

	psbt := getZombieMakeOffer(
		t, "alice", tempDir, resultsFileAlice, resultsFileBob,
		7_000_000,
	)

	txHex := getZombieSignOffer(t, "bob", tempDir, psbt)
	publishTx(t, txHex, backend)
}

func runZombieRecoveryLndCln(t *testing.T) {
	bobChannels := readChannelsJSON(t, "bob")
	bobIdentity := getNodeIdentityKey(t, "bob")
	rustyIdentity := getNodeIdentityKeyCln(t, "rusty")
	bobSweepAddr := randTaprootAddr(t)
	rustySweepAddr := randTaprootAddr(t)

	var bobRustyChannel *lnrpc.Channel
	for _, c := range bobChannels {
		if c.RemotePubkey == rustyIdentity {
			bobRustyChannel = c
		}
	}
	require.NotNil(t, bobRustyChannel, "bob-rusty channel not found")

	backend := connectBitcoind(t)
	addr := addressOfOutpoint(t, backend, bobRustyChannel.ChannelPoint)
	matchFile := makeMatchFile(
		t, "bob", "rusty", bobIdentity, rustyIdentity,
		bobRustyChannel.ChannelPoint,
		strconv.FormatUint(bobRustyChannel.ChanId, 10), addr,
		bobRustyChannel.Capacity,
	)

	resultsFileBob := getZombiePreparedKeys(
		t, "bob", tempDir, matchFile, bobSweepAddr,
	)
	resultsFileRusty := getZombiePreparedKeysCln(
		t, "rusty", tempDir, matchFile, rustySweepAddr,
	)

	psbt := getZombieMakeOffer(
		t, "bob", tempDir, resultsFileBob, resultsFileRusty,
		7_000_000,
	)

	txHex := getZombieSignOfferCln(t, "rusty", tempDir, bobIdentity, psbt)
	publishTx(t, txHex, backend)
}

func runZombieRecoveryClnLnd(t *testing.T) {
	charlieChannels := readChannelsJSON(t, "charlie")
	charlieIdentity := getNodeIdentityKey(t, "charlie")
	rustyIdentity := getNodeIdentityKeyCln(t, "rusty")
	charlieSweepAddr := randTaprootAddr(t)
	rustySweepAddr := randTaprootAddr(t)

	var rustyCharlieChannel *lnrpc.Channel
	for _, c := range charlieChannels {
		if c.RemotePubkey == rustyIdentity {
			rustyCharlieChannel = c
		}
	}
	require.NotNil(
		t, rustyCharlieChannel, "charlie-rusty channel not found",
	)

	backend := connectBitcoind(t)
	addr := addressOfOutpoint(t, backend, rustyCharlieChannel.ChannelPoint)
	matchFile := makeMatchFile(
		t, "charlie", "rusty", charlieIdentity, rustyIdentity,
		rustyCharlieChannel.ChannelPoint,
		strconv.FormatUint(rustyCharlieChannel.ChanId, 10), addr,
		rustyCharlieChannel.Capacity,
	)

	resultsFileCharlie := getZombiePreparedKeys(
		t, "charlie", tempDir, matchFile, charlieSweepAddr,
	)
	resultsFileRusty := getZombiePreparedKeysCln(
		t, "rusty", tempDir, matchFile, rustySweepAddr,
	)

	psbt := getZombieMakeOfferCln(
		t, "rusty", tempDir, resultsFileCharlie, resultsFileRusty,
		7_000_000,
	)

	txHex := getZombieSignOffer(t, "charlie", tempDir, psbt)
	publishTx(t, txHex, backend)
}

func runZombieRecoveryClnCln(t *testing.T) {
	niftyIdentity := getNodeIdentityKeyCln(t, "nifty")
	rustyChannels := readChannelsJSONCln(t, "rusty")
	rustyIdentity := getNodeIdentityKeyCln(t, "rusty")
	niftySweepAddr := randTaprootAddr(t)
	rustySweepAddr := randTaprootAddr(t)

	var rustyNiftyChannel *clnChannel
	for _, c := range rustyChannels {
		src := c.Source
		dst := c.Destination
		if (dst == niftyIdentity && src == rustyIdentity) ||
			(dst == rustyIdentity && src == niftyIdentity) {

			rustyNiftyChannel = &c
		}
	}
	require.NotNil(
		t, rustyNiftyChannel, "rusty-nifty channel not found",
	)

	backend := connectBitcoind(t)
	addr, op := addrAndOpFromShortChannelID(
		t, backend, rustyNiftyChannel.ShortID,
	)

	matchFile := makeMatchFile(
		t, "nifty", "rusty", niftyIdentity, rustyIdentity,
		op.String(), rustyNiftyChannel.ShortID, addr,
		rustyNiftyChannel.AmountMsat/1000,
	)

	resultsFileNifty := getZombiePreparedKeysCln(
		t, "nifty", tempDir, matchFile, niftySweepAddr,
	)
	resultsFileRusty := getZombiePreparedKeysCln(
		t, "rusty", tempDir, matchFile, rustySweepAddr,
	)

	psbt := getZombieMakeOfferCln(
		t, "rusty", tempDir, resultsFileNifty, resultsFileRusty,
		7_000_000,
	)

	txHex := getZombieSignOfferCln(t, "nifty", tempDir, rustyIdentity, psbt)
	publishTx(t, txHex, backend)
}
