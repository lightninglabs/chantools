package main

import (
	"crypto/sha256"
	"encoding/binary"
	"os"
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
	require.Equal(t, "present", report.BucketState)
	require.Equal(t, 2, report.Total)
	require.Equal(t, 1, report.V1OK)
	require.Equal(t, 1, report.Fatal)
	require.Equal(t, 1, report.ExactMatch)
	require.Equal(t, "exact_reported_crash_reproduced", report.Verdict)
	require.Equal(t, "clean_legacy_v1", report.Records[1].Legacy)
	require.Equal(t, before, fileSHA256(t, path))
}

func TestInspectWaitingProofsAbsentAndEmpty(t *testing.T) {
	absentPath := createWaitingProofTestDB(t, false)
	report, err := inspectWaitingProofDB(absentPath)
	require.NoError(t, err)
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

func fileSHA256(t *testing.T, path string) [sha256.Size]byte {
	t.Helper()

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	return sha256.Sum256(data)
}
