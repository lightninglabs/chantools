package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
	bbolt "go.etcd.io/bbolt"
)

func TestInspectWaitingProofsExactLegacyCrash(t *testing.T) {
	path := createWaitingProofTestDB(t, true)
	before := fileSHA256(t, path)

	report, err := inspectWaitingProofDB(path)
	require.NoError(t, err)
	require.NotNil(t, report.DBVersion)
	require.Equal(t, uint32(35), *report.DBVersion)
	require.Equal(t, "present", report.DBVersionStatus)
	require.Equal(t, "present", report.BucketState)
	require.Equal(t, 2, report.Total)
	require.Equal(t, 1, report.V1OK)
	require.Equal(t, 1, report.Fatal)
	require.Equal(t, 1, report.ExactMatch)
	require.Equal(t, "exact_reported_crash_reproduced", report.Verdict)
	require.Equal(t, "clean_legacy_v1", report.Records[1].Legacy)
	require.Equal(t, before, fileSHA256(t, path))
}

func TestInspectWaitingProofsMissingVersionAndTwoLegacyProofs(t *testing.T) {
	path := createWaitingProofMissingVersionTestDB(t)
	before := fileSHA256(t, path)

	report, err := inspectWaitingProofDB(path)
	require.NoError(t, err)
	require.Nil(t, report.DBVersion)
	require.Equal(t, "db_version_key_missing", report.DBVersionStatus)
	require.Equal(t, "present", report.BucketState)
	require.Equal(t, 2, report.Total)
	require.Zero(t, report.V1OK)
	require.Equal(t, 2, report.Fatal)
	require.Equal(t, 1, report.ExactMatch)
	require.Equal(t, "exact_reported_crash_reproduced", report.Verdict)

	for _, record := range report.Records {
		require.Equal(t, "clean_legacy_v1", record.Legacy)
		require.Equal(t, "legacy_length_key", record.KeyStatus)
	}

	require.Equal(t, before, fileSHA256(t, path))
}

func TestInspectWaitingProofsCommandOutput(t *testing.T) {
	path := createWaitingProofMissingVersionTestDB(t)

	cmd := newInspectWaitingProofsCommand()
	var output bytes.Buffer
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs([]string{"--channeldb", path})

	require.NoError(t, cmd.Execute())

	lines := output.String()
	require.Contains(
		t, lines,
		"db_version=unknown db_version_status=db_version_key_missing",
	)
	require.Contains(t, lines, "waitingproofs_bucket=present")
	require.Contains(t, lines, "legacy=clean_legacy_v1")
	require.Contains(t, lines, "key_status=\"legacy_length_key\"")
	require.Contains(t, lines, "error=\"invalid public key: unsupported format: 3d\"")
	require.Contains(
		t, lines,
		"summary total=2 v1_ok=0 v2_candidates=0 fatal=2 "+
			"exact_unsupported_format_3d=1",
	)
	require.Contains(t, lines, "verdict=exact_reported_crash_reproduced")
	require.Equal(t, 2, strings.Count(lines, "legacy=clean_legacy_v1"))
}

func TestInspectWaitingProofsAbsentAndEmpty(t *testing.T) {
	absentPath := createWaitingProofTestDB(t, false)
	report, err := inspectWaitingProofDB(absentPath)
	require.NoError(t, err)
	require.Equal(t, "present", report.DBVersionStatus)
	require.Equal(t, "absent", report.BucketState)
	require.Equal(t, "waiting_proof_store_ruled_out", report.Verdict)
	require.Zero(t, report.Total)

	emptyPath := t.TempDir() + "/channel.db"
	db, err := bbolt.Open(emptyPath, dbFilePermission, nil)
	require.NoError(t, err)
	require.NoError(t, db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucket(waitingProofsBucket)
		return err
	}))
	require.NoError(t, db.Close())

	report, err = inspectWaitingProofDB(emptyPath)
	require.NoError(t, err)
	require.Equal(t, "metadata_bucket_missing", report.DBVersionStatus)
	require.Equal(t, "empty", report.BucketState)
	require.Equal(t, "waiting_proof_store_ruled_out", report.Verdict)
	require.Zero(t, report.Total)
}

func TestInspectWaitingProofsTypeFlipIsNotLegacy(t *testing.T) {
	key := make([]byte, typedWaitingProofKeyLen)
	value := make([]byte, 2+announceSignatures1Len)
	value[0], value[1], value[2], value[3] = 1, 1, 1, 0x3d

	record := classifyWaitingProof(key, value)
	require.True(t, record.Fatal)
	require.Equal(
		t, "invalid public key: unsupported format: 3d",
		record.DecodeError,
	)
	require.Equal(t, "not_legacy", record.Legacy)
	require.Equal(t, "typed key/value proof types disagree", record.KeyStatus)
}

func TestInspectWaitingProofsUnknownAndTruncated(t *testing.T) {
	tests := []struct {
		name  string
		value []byte
		err   string
	}{
		{
			name:  "truncated header",
			value: []byte{0},
			err:   "unexpected EOF reading proof type/side",
		},
		{
			name:  "unknown type",
			value: []byte{9, 0},
			err:   "unknown waiting proof type: 9",
		},
		{
			name:  "truncated v1",
			value: []byte{0, 0},
			err:   "unexpected EOF decoding AnnounceSignatures1",
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			record := classifyWaitingProof(nil, test.value)
			require.True(t, record.Fatal)
			require.Equal(t, test.err, record.DecodeError)
		})
	}
}

func createWaitingProofTestDB(t *testing.T, withBucket bool) string {
	t.Helper()

	path := t.TempDir() + "/channel.db"
	db, err := bbolt.Open(path, dbFilePermission, nil)
	require.NoError(t, err)
	require.NoError(t, db.Update(func(tx *bbolt.Tx) error {
		meta, err := tx.CreateBucket(metadataBucket)
		if err != nil {
			return err
		}
		version := make([]byte, 4)
		binary.BigEndian.PutUint32(version, 35)
		if err := meta.Put(dbVersionKey, version); err != nil {
			return err
		}
		if !withBucket {
			return nil
		}

		bucket, err := tx.CreateBucket(waitingProofsBucket)
		if err != nil {
			return err
		}

		goodKey := make([]byte, typedWaitingProofKeyLen)
		binary.BigEndian.PutUint64(goodKey[1:9], 111)
		goodKey[9] = 1
		goodValue := make([]byte, 2+announceSignatures1Len)
		goodValue[1] = 1
		binary.BigEndian.PutUint64(goodValue[34:42], 111)
		if err := bucket.Put(goodKey, goodValue); err != nil {
			return err
		}

		const scid = uint64(222)
		badKey := make([]byte, legacyWaitingProofKeyLen)
		binary.BigEndian.PutUint64(badKey[:8], scid)
		badKey[8] = 1
		badValue := make([]byte, 1+announceSignatures1Len)
		badValue[0], badValue[1], badValue[2], badValue[3] = 1, 1, 1, 0x3d
		binary.BigEndian.PutUint64(badValue[33:41], scid)

		return bucket.Put(badKey, badValue)
	}))
	require.NoError(t, db.Close())

	return path
}

func createWaitingProofMissingVersionTestDB(t *testing.T) string {
	t.Helper()

	path := t.TempDir() + "/channel.db"
	db, err := bbolt.Open(path, dbFilePermission, nil)
	require.NoError(t, err)
	require.NoError(t, db.Update(func(tx *bbolt.Tx) error {
		_, err := tx.CreateBucket(metadataBucket)
		if err != nil {
			return err
		}

		bucket, err := tx.CreateBucket(waitingProofsBucket)
		if err != nil {
			return err
		}

		localKey := make([]byte, legacyWaitingProofKeyLen)
		binary.BigEndian.PutUint64(localKey[:8], 0x0e9a2f0005eb0001)
		localValue := make([]byte, 1+announceSignatures1Len)
		localValue[1], localValue[2], localValue[3] = 0xd9, 0xd0, 0x75
		binary.BigEndian.PutUint64(localValue[33:41], 0x0e9a2f0005eb0001)
		if err := bucket.Put(localKey, localValue); err != nil {
			return err
		}

		remoteKey := make([]byte, legacyWaitingProofKeyLen)
		binary.BigEndian.PutUint64(remoteKey[:8], 0x0e730200077f0001)
		remoteKey[8] = 1
		remoteValue := make([]byte, 1+announceSignatures1Len)
		remoteValue[0] = 1
		remoteValue[1], remoteValue[2], remoteValue[3] = 0x5e, 0x72, 0x3d
		binary.BigEndian.PutUint64(remoteValue[33:41], 0x0e730200077f0001)

		return bucket.Put(remoteKey, remoteValue)
	}))
	require.NoError(t, db.Close())

	return path
}

func fileSHA256(t *testing.T, path string) [sha256.Size]byte {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return sha256.Sum256(data)
}
