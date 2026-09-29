package itest

import (
	"bytes"
	"context"
	"encoding/hex"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/btcsuite/btcd/wire"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/channeldb"
	graphdb "github.com/lightningnetwork/lnd/graph/db"
	"github.com/lightningnetwork/lnd/graph/db/models"
	"github.com/lightningnetwork/lnd/lnwire"
	paymentsdb "github.com/lightningnetwork/lnd/payments/db"
	"github.com/stretchr/testify/require"
)

// The commands in this file all modify an lnd channel.db in place. To keep the
// test cases independent of each other and of the shared test network data,
// every test case works on its own copy of the (stopped) node's channel.db and
// verifies the result by re-opening that copy with lnd's own DB code, the same
// way lnd would when it is started again after running chantools.

// copyChannelDB copies the channel.db of the given (stopped) lnd node into a
// temporary directory and returns the path of the copy.
//
//nolint:unparam
func copyChannelDB(t *testing.T, node string) string {
	t.Helper()

	src, err := os.Open(fmt.Sprintf(channelDBFilePattern, node))
	require.NoError(t, err)
	defer func() { _ = src.Close() }()

	dstPath := filepath.Join(t.TempDir(), "channel.db")
	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_WRONLY, 0600)
	require.NoError(t, err)
	defer func() { _ = dst.Close() }()

	_, err = io.Copy(dst, src)
	require.NoError(t, err)

	return dstPath
}

// withChannelDB opens the channel DB at the given path, hands it to the
// callback and closes it again. The DB must be closed before chantools is
// invoked on it, since bbolt holds an exclusive file lock while it is open.
func withChannelDB(t *testing.T, dbPath string, cb func(db *channeldb.DB,
	graph *graphdb.KVStore)) {

	t.Helper()

	db, _, err := lnd.OpenDB(dbPath, false)
	require.NoError(t, err)
	defer func() { require.NoError(t, db.Close()) }()

	// We intentionally open the graph store without the NoMigration
	// option, just like lnd does on startup. This re-creates any top-level
	// graph buckets that chantools might have removed.
	graph, err := graphdb.NewKVStore(db.Backend)
	require.NoError(t, err)

	cb(db, graph)
}

// runChannelDBCmd runs a chantools sub command that doesn't require any user
// interaction and asserts that it succeeds within the default timeout. These
// commands only work on a local DB file, so a command that doesn't finish in
// time is stuck (for example on a lock or an unbuffered channel) rather than
// slow, and we want to fail fast instead of waiting for the test timeout.
func runChannelDBCmd(t *testing.T, args ...string) {
	t.Helper()

	proc := StartChantools(t, append([]string{"--regtest"}, args...)...)

	// We can't use proc.ReadAllOutput here, as that asserts with require,
	// which must only be called from the test's own goroutine.
	done := make(chan struct{})
	go func() {
		defer close(done)
		out, _ := io.ReadAll(proc.stdout)
		log.Debugf("[CHANTOOLS]: %s", bytes.TrimSpace(out))
	}()

	select {
	case <-done:
		proc.Wait(t)

	case <-time.After(longTimeout):
		proc.Kill(t)
		require.Failf(t, "chantools command timed out", "command %v "+
			"did not finish within %v", args, longTimeout)
	}
}

// countPayments returns the number of succeeded and failed payments, as well
// as the total number of payments in the given DB.
func countPayments(t *testing.T, dbPath string) (int, int, int) {
	t.Helper()

	var succeeded, failed, total int
	withChannelDB(t, dbPath, func(db *channeldb.DB, _ *graphdb.KVStore) {
		paymentsDB, err := paymentsdb.NewKVStore(db.Backend)
		require.NoError(t, err)

		payments, err := paymentsDB.FetchPayments()
		require.NoError(t, err)

		for _, p := range payments {
			switch p.Status {
			case paymentsdb.StatusSucceeded:
				succeeded++

			case paymentsdb.StatusFailed:
				failed++
			}
		}
		total = len(payments)
	})

	return succeeded, failed, total
}

// graphEdges returns all channel edges in the graph, keyed by their SCID.
func graphEdges(ctx context.Context, t *testing.T,
	graph *graphdb.KVStore) map[uint64]*models.ChannelEdgeInfo {

	t.Helper()

	edges := make(map[uint64]*models.ChannelEdgeInfo)
	err := graph.ForEachChannel(
		ctx, lnwire.GossipVersion1,
		func(info *models.ChannelEdgeInfo, _,
			_ *models.ChannelEdgePolicy) error {

			edges[info.ChannelID] = info

			return nil
		}, func() {
			clear(edges)
		},
	)
	require.NoError(t, err)

	return edges
}

// runDeletePayments tests the deletepayments command, first only deleting the
// failed payments, then deleting all the rest.
func runDeletePayments(t *testing.T) {
	dbPath := copyChannelDB(t, "alice")

	// The test network setup sends successful payments and also creates at
	// least one failed payment for Alice.
	succeeded, failed, total := countPayments(t, dbPath)
	require.Positive(t, succeeded)
	require.Positive(t, failed)

	// Deleting only the failed payments must keep all successful ones.
	runChannelDBCmd(
		t, "deletepayments", "--failedonly", "--channeldb", dbPath,
	)
	succeededAfter, failedAfter, totalAfter := countPayments(t, dbPath)
	require.Equal(t, succeeded, succeededAfter)
	require.Zero(t, failedAfter)
	require.Equal(t, total-failed, totalAfter)

	// Without the flag, all (completed) payments are removed.
	runChannelDBCmd(t, "deletepayments", "--channeldb", dbPath)
	_, _, totalAfter = countPayments(t, dbPath)
	require.Zero(t, totalAfter)
}

// runDropChannelGraphSingle tests removing a single channel from the graph
// with the dropchannelgraph command.
func runDropChannelGraphSingle(t *testing.T) {
	dbPath := copyChannelDB(t, "alice")
	aliceIdentity := readNodeIdentityFromFile(t, "alice")

	// We remove the Charlie-Dave channel, which Alice only knows about from
	// gossip.
	var (
		edgesBefore map[uint64]*models.ChannelEdgeInfo
		targetSCID  uint64
	)
	targetChan := findChannel(t, "charlie", readNodeIdentityFromFile(
		t, "dave",
	))
	withChannelDB(t, dbPath, func(_ *channeldb.DB, g *graphdb.KVStore) {
		edgesBefore = graphEdges(t.Context(), t, g)
		targetSCID = targetChan.ChanId
		require.Contains(t, edgesBefore, targetSCID)
	})

	runChannelDBCmd(
		t, "dropchannelgraph", "--channeldb", dbPath,
		"--node_identity_key", aliceIdentity,
		"--single_channel", strconv.FormatUint(targetSCID, 10),
	)

	withChannelDB(t, dbPath, func(_ *channeldb.DB, g *graphdb.KVStore) {
		ctx := t.Context()

		// Only the target channel was removed.
		edgesAfter := graphEdges(ctx, t, g)
		require.Len(t, edgesAfter, len(edgesBefore)-1)
		require.NotContains(t, edgesAfter, targetSCID)

		_, _, _, err := g.FetchChannelEdgesByID(
			ctx, lnwire.GossipVersion1, targetSCID,
		)
		require.ErrorIs(t, err, graphdb.ErrEdgeNotFound)

		// The channel is intentionally NOT marked as a zombie, so lnd
		// can learn about it again from gossip.
		isZombie, _, _, err := g.IsZombieEdge(
			ctx, lnwire.GossipVersion1, targetSCID,
		)
		require.NoError(t, err)
		require.False(t, isZombie)
	})
}

// runDropChannelGraphFull tests wiping the whole graph with the
// dropchannelgraph command and then re-adding the node's own channels with the
// --fix_only flag.
func runDropChannelGraphFull(t *testing.T) {
	dbPath := copyChannelDB(t, "alice")
	aliceIdentity := readNodeIdentityFromFile(t, "alice")
	aliceKey, err := hex.DecodeString(aliceIdentity)
	require.NoError(t, err)

	var (
		numEdgesBefore int
		openChannels   []*channeldb.OpenChannel
	)
	withChannelDB(t, dbPath, func(db *channeldb.DB, g *graphdb.KVStore) {
		numEdgesBefore = len(graphEdges(t.Context(), t, g))

		openChannels, err = db.ChannelStateDB().FetchAllOpenChannels()
		require.NoError(t, err)
	})
	require.NotEmpty(t, openChannels)
	require.Greater(t, numEdgesBefore, len(openChannels))

	// Dropping the graph removes all edges and nodes, including our own
	// source node.
	runChannelDBCmd(
		t, "dropchannelgraph", "--channeldb", dbPath,
		"--node_identity_key", aliceIdentity,
	)
	withChannelDB(t, dbPath, func(db *channeldb.DB, g *graphdb.KVStore) {
		require.Empty(t, graphEdges(t.Context(), t, g))

		_, err := g.SourceNode(
			t.Context(), lnwire.GossipVersion1,
		)
		require.ErrorIs(t, err, graphdb.ErrSourceNodeNotSet)

		// The actual channel state must not be touched by this.
		channels, err := db.ChannelStateDB().FetchAllOpenChannels()
		require.NoError(t, err)
		require.Len(t, channels, len(openChannels))
	})

	// Re-adding our own channels must produce exactly one edge with our
	// own policy for each of our open channels.
	runChannelDBCmd(
		t, "dropchannelgraph", "--channeldb", dbPath,
		"--node_identity_key", aliceIdentity, "--fix_only",
	)
	withChannelDB(t, dbPath, func(_ *channeldb.DB, g *graphdb.KVStore) {
		ctx := t.Context()

		edges := graphEdges(ctx, t, g)
		require.Len(t, edges, len(openChannels))

		for _, c := range openChannels {
			scid := c.ShortChannelID.ToUint64()
			require.Contains(t, edges, scid)

			info, policy1, policy2, err := g.FetchChannelEdgesByID(
				ctx, lnwire.GossipVersion1, scid,
			)
			require.NoError(t, err)
			require.Equal(t, c.FundingOutpoint, info.ChannelPoint)
			require.Equal(t, c.Capacity, info.Capacity)

			remoteKey := c.IdentityPub.SerializeCompressed()
			node1, node2 := info.NodeKey1Bytes, info.NodeKey2Bytes

			// Only our own direction of the channel has a policy.
			// Which one that is depends on the lexicographical
			// order of the two node keys.
			ourPolicy, theirPolicy := policy1, policy2
			if bytes.Compare(aliceKey, remoteKey) > 0 {
				ourPolicy, theirPolicy = policy2, policy1
				require.Equal(t, aliceKey, node2[:])
				require.Equal(t, remoteKey, node1[:])
			} else {
				require.Equal(t, aliceKey, node1[:])
				require.Equal(t, remoteKey, node2[:])
			}
			require.NotNil(t, ourPolicy)
			require.Nil(t, theirPolicy)
			require.Equal(t, scid, ourPolicy.ChannelID)
		}
	})
}

// runDropGraphZombies tests the dropgraphzombies command.
func runDropGraphZombies(t *testing.T) {
	dbPath := copyChannelDB(t, "alice")
	ctx := t.Context()

	// We need a zombie channel in the graph to begin with. We use lnd's own
	// code path for that, by deleting the Charlie-Dave channel from the
	// graph and marking it as a zombie, which is what lnd does when it
	// prunes channels that haven't seen updates for too long.
	targetChan := findChannel(t, "charlie", readNodeIdentityFromFile(
		t, "dave",
	))
	targetSCID := targetChan.ChanId

	var numEdgesBefore int
	withChannelDB(t, dbPath, func(_ *channeldb.DB, g *graphdb.KVStore) {
		_, err := g.DeleteChannelEdges(
			ctx, lnwire.GossipVersion1, false, true, targetSCID,
		)
		require.NoError(t, err)

		isZombie, _, _, err := g.IsZombieEdge(
			ctx, lnwire.GossipVersion1, targetSCID,
		)
		require.NoError(t, err)
		require.True(t, isZombie)

		numEdgesBefore = len(graphEdges(ctx, t, g))
	})

	runChannelDBCmd(t, "dropgraphzombies", "--channeldb", dbPath)

	withChannelDB(t, dbPath, func(_ *channeldb.DB, g *graphdb.KVStore) {
		// The zombie index is gone (and re-created empty when lnd
		// opens the graph again), so the channel can be re-learned
		// from gossip.
		isZombie, _, _, err := g.IsZombieEdge(
			ctx, lnwire.GossipVersion1, targetSCID,
		)
		require.NoError(t, err)
		require.False(t, isZombie)

		_, _, _, err = g.FetchChannelEdgesByID(
			ctx, lnwire.GossipVersion1, targetSCID,
		)
		require.ErrorIs(t, err, graphdb.ErrEdgeNotFound)

		// All the other (non-zombie) channels are still there.
		require.Len(t, graphEdges(ctx, t, g), numEdgesBefore)
	})
}

// runRemoveChannel tests the removechannel command.
func runRemoveChannel(t *testing.T) {
	dbPath := copyChannelDB(t, "alice")

	targetChan := findChannel(t, "alice", readNodeIdentityFromFile(
		t, "bob",
	))
	chanPoint, err := wire.NewOutPointFromString(targetChan.ChannelPoint)
	require.NoError(t, err)

	var numOpenBefore int
	withChannelDB(t, dbPath, func(db *channeldb.DB, _ *graphdb.KVStore) {
		channels, err := db.ChannelStateDB().FetchAllOpenChannels()
		require.NoError(t, err)
		numOpenBefore = len(channels)

		_, err = db.ChannelStateDB().FetchChannel(*chanPoint)
		require.NoError(t, err)
	})

	runChannelDBCmd(
		t, "removechannel", "--channeldb", dbPath,
		"--channel", targetChan.ChannelPoint,
	)

	withChannelDB(t, dbPath, func(db *channeldb.DB, _ *graphdb.KVStore) {
		chanDB := db.ChannelStateDB()

		_, err := chanDB.FetchChannel(*chanPoint)
		require.ErrorIs(t, err, channeldb.ErrChannelNotFound)

		channels, err := chanDB.FetchAllOpenChannels()
		require.NoError(t, err)
		require.Len(t, channels, numOpenBefore-1)

		// The channel is recorded as abandoned in the closed channels.
		summary, err := chanDB.FetchClosedChannel(chanPoint)
		require.NoError(t, err)
		require.Equal(t, channeldb.Abandoned, summary.CloseType)
	})
}
