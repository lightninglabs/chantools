package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	bbolt "go.etcd.io/bbolt"
)

func TestRepairWaitingProofsDryRun(t *testing.T) {
	path := createWaitingProofMissingVersionTestDB(t)
	before := fileSHA256(t, path)

	report, err := repairWaitingProofDB(path, false)
	require.NoError(t, err)
	require.True(t, report.DryRun)
	require.Equal(t, "db_version_key_missing", report.DBVersionStatus)
	require.Equal(t, "present", report.BucketState)
	require.Equal(t, 2, report.TotalRecords)
	require.Equal(t, 2, report.LegacyClean)
	require.Equal(t, 2, report.WouldMigrate)
	require.Zero(t, report.Migrated)
	require.Zero(t, report.DeletedLegacy)
	require.Empty(t, report.BackupPath)

	require.Equal(t, before, fileSHA256(t, path))
}

func TestRepairWaitingProofsCommit(t *testing.T) {
	path := createWaitingProofMissingVersionTestDB(t)
	before := fileSHA256(t, path)

	report, err := repairWaitingProofDB(path, true)
	require.NoError(t, err)
	require.False(t, report.DryRun)
	require.Equal(t, "db_version_key_missing", report.DBVersionStatus)
	require.Equal(t, "present", report.BucketState)
	require.Equal(t, 2, report.TotalRecords)
	require.Equal(t, 2, report.LegacyClean)
	require.Equal(t, 2, report.WouldMigrate)
	require.Equal(t, 2, report.Migrated)
	require.Equal(t, 2, report.DeletedLegacy)
	require.FileExists(t, report.BackupPath)
	require.Equal(t, before, fileSHA256(t, report.BackupPath))

	inspect, err := inspectWaitingProofDB(path)
	require.NoError(t, err)
	require.Nil(t, inspect.DBVersion)
	require.Equal(t, "db_version_key_missing", inspect.DBVersionStatus)
	require.Equal(t, "present", inspect.BucketState)
	require.Equal(t, 2, inspect.Total)
	require.Equal(t, 2, inspect.V1OK)
	require.Zero(t, inspect.Fatal)
	require.Zero(t, inspect.ExactMatch)
	require.Equal(t, "reported_crash_not_found", inspect.Verdict)

	require.NotEqual(t, before, fileSHA256(t, path))
}

func TestRepairWaitingProofsCommandOutput(t *testing.T) {
	path := createWaitingProofMissingVersionTestDB(t)

	cmd := newRepairWaitingProofsCommand()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--channeldb", path, "--commit"})

	require.NoError(t, cmd.Execute())

	lines := output.String()
	require.Contains(t, lines, "dry_run=false")
	require.Contains(t, lines, "backup=")
	require.Contains(t, lines, "db_version_status=db_version_key_missing")
	require.Contains(
		t, lines,
		"summary total=2 clean_legacy=2 typed=0 other=0 "+
			"would_migrate=2 migrated=2 deleted_legacy=2",
	)
	require.Contains(t, lines, "verdict=repaired")
}

func TestRepairWaitingProofsConflict(t *testing.T) {
	path := createWaitingProofMissingVersionTestDB(t)

	err := withWaitingProofBucket(path, func(bucket *bbolt.Bucket) error {
		legacyKey := make([]byte, legacyWaitingProofKeyLen)
		binary.BigEndian.PutUint64(legacyKey[:8], 0x0e730200077f0001)
		legacyKey[8] = 1

		typedKey := make([]byte, typedWaitingProofKeyLen)
		typedKey[0] = waitingProofV1Type
		copy(typedKey[1:9], legacyKey[:8])
		typedKey[9] = legacyKey[8]

		return bucket.Put(typedKey, []byte{0, 1, 99})
	})
	require.NoError(t, err)

	_, err = repairWaitingProofDB(path, true)
	require.ErrorContains(t, err, "already exists with different value")
}

func TestRepairWaitingProofsNoop(t *testing.T) {
	path := createWaitingProofMissingVersionTestDB(t)

	_, err := repairWaitingProofDB(path, true)
	require.NoError(t, err)

	report, err := repairWaitingProofDB(path, true)
	require.NoError(t, err)
	require.Zero(t, report.WouldMigrate)
	require.Zero(t, report.Migrated)
	require.Zero(t, report.DeletedLegacy)
	require.Equal(t, 2, report.TypedRecords)
}

func TestRepairWaitingProofsRequiresChannelDB(t *testing.T) {
	cmd := newRepairWaitingProofsCommand()
	cmd.SetArgs(nil)

	err := cmd.Execute()
	require.ErrorContains(t, err, "channel DB is required")
}

func TestRepairWaitingProofsCreatesBackupNextToDB(t *testing.T) {
	path := createWaitingProofMissingVersionTestDB(t)

	report, err := repairWaitingProofDB(path, true)
	require.NoError(t, err)
	require.True(t, strings.HasPrefix(report.BackupPath, path))
	require.FileExists(t, report.BackupPath)

	_, err = os.Stat(report.BackupPath)
	require.NoError(t, err)
}

func withWaitingProofBucket(path string,
	fn func(bucket *bbolt.Bucket) error) error {

	db, err := bbolt.Open(path, dbFilePermission, nil)
	if err != nil {
		return err
	}
	defer func() { _ = db.Close() }()

	return db.Update(func(tx *bbolt.Tx) error {
		return fn(tx.Bucket(waitingProofsBucket))
	})
}
