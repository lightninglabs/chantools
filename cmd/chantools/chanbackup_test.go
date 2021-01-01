package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	backupContent = "FundingOutpoint: (string) (len=66) \"10279f626196340" +
		"58b6133cb7ac6c1693a8e6df7caa91c6263ca3d0bf704ad4d:0\""
)

func TestChanBackupAndDumpBackup(t *testing.T) {
	h := newHarness(t)

	// Create a channel backup from a channel DB file.
	makeBackup := &chanBackupCommand{
		ChannelDB: h.testdataFile("channel.db"),
		MultiFile: h.tempFile("extracted.backup"),
		rootKey:   &rootKey{RootKey: rootKeyAezeed},
	}

	err := makeBackup.Execute(nil, nil)
	require.NoError(t, err)

	// Decrypt and dump the channel backup file.
	dumpBackup := &dumpBackupCommand{
		MultiFile: makeBackup.MultiFile,
		rootKey:   &rootKey{RootKey: rootKeyAezeed},
	}

	err = dumpBackup.Execute(nil, nil)
	require.NoError(t, err)

	h.assertLogContains(backupContent)
}
