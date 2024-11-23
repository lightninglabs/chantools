package main

import (
	"testing"

	"github.com/lightninglabs/chantools/lnd"
	"github.com/stretchr/testify/require"
)

func TestCompactDBAndDumpChannels(t *testing.T) {
	h := newHarness(t)

	// Compact the test DB.
	compact := &compactDBCommand{
		SourceDB: h.testdataFile("channel.db"),
		DestDB:   h.tempFile("compacted.db"),
	}

	err := compact.Execute(nil, nil)
	require.NoError(t, err)

	require.FileExists(t, compact.DestDB)

	// Compacting small DBs actually increases the size slightly. But we
	// just want to make sure the contents match.
	require.GreaterOrEqual(
		t, h.fileSize(compact.DestDB), h.fileSize(compact.SourceDB),
	)

	// Compare the content of the source and destination DB by looking at
	// the logged dump.
	dump := &dumpChannelsCommand{
		ChannelDB: compact.SourceDB,
		dbConfig: &lnd.DB{
			Backend: "bolt",
			Bolt:    &lnd.Bolt{},
		},
	}
	h.clearLog()
	err = dump.Execute(nil, nil)
	require.NoError(t, err)
	sourceDump := h.getLog()

	h.clearLog()
	dump.ChannelDB = compact.DestDB
	dump.dbConfig = &lnd.DB{
		Backend: "bolt",
		Bolt: &lnd.Bolt{
			Name: "compacted.db",
		},
	}
	err = dump.Execute(nil, nil)
	require.NoError(t, err)
	destDump := h.getLog()

	h.assertLogEqual(sourceDump, destDump)
}
