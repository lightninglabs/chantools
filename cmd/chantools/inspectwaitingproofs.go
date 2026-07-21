package main

import (
	"bytes"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/spf13/cobra"
	bbolt "go.etcd.io/bbolt"
)

var (
	waitingProofsBucket = []byte("waitingproofs")
	metadataBucket      = []byte("metadata")
	dbVersionKey        = []byte("dbp")
)

const (
	waitingProofV1Type = byte(0)
	waitingProofV2Type = byte(1)

	legacyWaitingProofKeyLen = 9
	typedWaitingProofKeyLen  = 10
	announceSignatures1Len   = 168

	waitingProofClassV1OK        = "v1_ok"
	waitingProofClassV2Candidate = "v2_candidate"
	waitingProofClassDecodeError = "decode_error"
)

type inspectWaitingProofsCommand struct {
	ChannelDB string
	JSON      bool

	cmd *cobra.Command
}

func newInspectWaitingProofsCommand() *cobra.Command {
	cc := &inspectWaitingProofsCommand{}
	cc.cmd = &cobra.Command{
		Use: "inspectwaitingproofs",
		Short: "Inspect lnd waiting proofs for startup-fatal records " +
			"without modifying the database",
		Long: `This command opens an offline copy of an lnd channel.db in
strictly read-only mode and inspects the waitingproofs bucket. It identifies
records that the lnd v0.21 typed waiting-proof decoder would reject, including
legacy remote proofs that are misread as V2 MuSig2 nonces.

Always stop lnd and create a copy of channel.db first. This command does not
repair or delete anything. Do not share channel.db because it contains
sensitive channel state.`,
		Example: `chantools inspectwaitingproofs \
	--channeldb /tmp/channel.db.waitingproof-check`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.ChannelDB, "channeldb", "", "offline copy of the lnd "+
			"channel.db file to inspect",
	)
	cc.cmd.Flags().BoolVar(
		&cc.JSON, "json", false, "print the inspection report as JSON",
	)

	return cc.cmd
}

func (c *inspectWaitingProofsCommand) Execute(cmd *cobra.Command,
	_ []string) error {

	if c.ChannelDB == "" {
		return errors.New("channel DB is required")
	}

	report, err := inspectWaitingProofDB(c.ChannelDB)
	if err != nil {
		return err
	}

	if c.JSON {
		encoder := json.NewEncoder(cmd.OutOrStdout())
		encoder.SetIndent("", "  ")
		return encoder.Encode(report)
	}

	writeWaitingProofReport(cmd.OutOrStdout(), report)

	return nil
}

type waitingProofReport struct {
	DBVersion    *uint32              `json:"db_version,omitempty"`
	BucketState  string               `json:"bucket_state"`
	Verdict      string               `json:"verdict"`
	Records      []waitingProofRecord `json:"records,omitempty"`
	Total        int                  `json:"total"`
	V1OK         int                  `json:"v1_ok"`
	V2Candidates int                  `json:"v2_candidates"`
	Fatal        int                  `json:"fatal"`
	ExactMatch   int                  `json:"exact_unsupported_format_3d"`
}

type waitingProofRecord struct {
	Key            string `json:"key"`
	KeyLength      int    `json:"key_length"`
	ValueLength    int    `json:"value_length"`
	ValuePrefix    string `json:"value_prefix"`
	Classification string `json:"classification"`
	Fatal          bool   `json:"fatal"`
	DecodeError    string `json:"decode_error,omitempty"`
	Legacy         string `json:"legacy,omitempty"`
	KeyStatus      string `json:"key_status,omitempty"`
}

func inspectWaitingProofDB(path string) (*waitingProofReport, error) {
	db, err := bbolt.Open(path, dbFilePermission, &bbolt.Options{
		ReadOnly: true,
		Timeout:  5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("error opening channel DB read-only: %w", err)
	}
	defer func() { _ = db.Close() }()

	report := &waitingProofReport{
		BucketState: "absent",
	}
	err = db.View(func(tx *bbolt.Tx) error {
		meta := tx.Bucket(metadataBucket)
		if meta != nil {
			versionBytes := meta.Get(dbVersionKey)
			if len(versionBytes) == 4 {
				version := binary.BigEndian.Uint32(versionBytes)
				report.DBVersion = &version
			}
		}

		bucket := tx.Bucket(waitingProofsBucket)
		if bucket == nil {
			return nil
		}

		report.BucketState = "empty"
		return bucket.ForEach(func(k, v []byte) error {
			if v == nil {
				return nil
			}

			report.BucketState = "present"
			record := classifyWaitingProof(k, v)
			report.Records = append(report.Records, record)
			report.Total++

			switch record.Classification {
			case waitingProofClassV1OK:
				report.V1OK++
			case waitingProofClassV2Candidate:
				report.V2Candidates++
			}
			if record.Fatal {
				report.Fatal++
			}
			if record.DecodeError ==
				"invalid public key: unsupported format: 3d" {

				report.ExactMatch++
			}

			return nil
		})
	})
	if err != nil {
		return nil, fmt.Errorf("error reading channel DB: %w", err)
	}
	report.Verdict = waitingProofVerdict(report)

	return report, nil
}

func classifyWaitingProof(k, v []byte) waitingProofRecord {
	record := waitingProofRecord{
		Key:         hex.EncodeToString(k),
		KeyLength:   len(k),
		ValueLength: len(v),
		ValuePrefix: hex.EncodeToString(v[:min(4, len(v))]),
	}

	if len(v) < 2 {
		record.Classification = waitingProofClassDecodeError
		record.Fatal = true
		record.DecodeError = "unexpected EOF reading proof type/side"
		return record
	}

	switch v[0] {
	case waitingProofV1Type:
		return classifyV1WaitingProof(record, k, v)

	case waitingProofV2Type:
		return classifyV2WaitingProof(record, k, v)

	default:
		record.Classification = waitingProofClassDecodeError
		record.Fatal = true
		record.DecodeError = fmt.Sprintf(
			"unknown waiting proof type: %d", v[0],
		)
		return record
	}
}

func classifyV1WaitingProof(record waitingProofRecord, k,
	v []byte) waitingProofRecord {

	if len(v) < 2+announceSignatures1Len {
		record.Classification = waitingProofClassDecodeError
		record.Fatal = true
		record.DecodeError = "unexpected EOF decoding AnnounceSignatures1"
		return record
	}

	record.Classification = waitingProofClassV1OK
	if len(k) != typedWaitingProofKeyLen {
		record.KeyStatus = fmt.Sprintf(
			"unexpected typed V1 key length %d", len(k),
		)
		return record
	}

	// Typed V1 key: [type(1), scid(8), isRemote(1)]. The SCID in the
	// value follows [type(1), isRemote(1), channelID(32)].
	if k[0] != waitingProofV1Type || k[9] != v[1] ||
		!bytes.Equal(k[1:9], v[34:42]) {

		record.KeyStatus = "typed V1 key does not match value"
	} else {
		record.KeyStatus = "typed V1 key matches value"
	}

	return record
}

func classifyV2WaitingProof(record waitingProofRecord, k,
	v []byte) waitingProofRecord {

	record.Legacy = classifyLegacyWaitingProof(k, v)
	record.KeyStatus = classifyV2WaitingProofKey(k)

	if len(v) < 3 {
		record.Classification = waitingProofClassDecodeError
		record.Fatal = true
		record.DecodeError = "unexpected EOF reading V2 nonce presence"
		return record
	}

	if v[2] == 0 {
		record.Classification = waitingProofClassV2Candidate
		return record
	}
	if len(v) < 3+btcec.PubKeyBytesLenCompressed {
		record.Classification = waitingProofClassDecodeError
		record.Fatal = true
		record.DecodeError = "unexpected EOF reading V2 combined nonce"
		return record
	}

	_, err := btcec.ParsePubKey(v[3 : 3+btcec.PubKeyBytesLenCompressed])
	if err != nil {
		record.Classification = waitingProofClassDecodeError
		record.Fatal = true
		record.DecodeError = err.Error()
		return record
	}

	record.Classification = waitingProofClassV2Candidate
	return record
}

func classifyV2WaitingProofKey(k []byte) string {
	switch len(k) {
	case legacyWaitingProofKeyLen:
		return "legacy_length_key"

	case typedWaitingProofKeyLen:
		if k[0] != waitingProofV2Type {
			return "typed key/value proof types disagree"
		}

		return "typed V2 key"

	default:
		return fmt.Sprintf("unexpected key length %d", len(k))
	}
}

func classifyLegacyWaitingProof(k, v []byte) string {
	if len(k) != legacyWaitingProofKeyLen ||
		len(v) < 1+announceSignatures1Len ||
		(v[0] != 0 && v[0] != 1) {

		return "not_legacy"
	}

	// Legacy key: [scid(8), isRemote(1)]. The SCID in the legacy value
	// follows [isRemote(1), channelID(32)].
	if k[8] != v[0] || !bytes.Equal(k[:8], v[33:41]) {
		return "legacy_shape_key_mismatch"
	}

	return "clean_legacy_v1"
}

func writeWaitingProofReport(w io.Writer, report *waitingProofReport) {
	if report.DBVersion == nil {
		_, _ = fmt.Fprintln(w, "db_version=unknown")
	} else {
		_, _ = fmt.Fprintf(w, "db_version=%d\n", *report.DBVersion)
	}
	_, _ = fmt.Fprintf(w, "waitingproofs_bucket=%s\n", report.BucketState)

	for _, record := range report.Records {
		_, _ = fmt.Fprintf(
			w, "[%s] key=%s key_len=%d value_len=%d prefix=%s",
			record.Classification, record.Key, record.KeyLength,
			record.ValueLength, record.ValuePrefix,
		)
		if record.DecodeError != "" {
			_, _ = fmt.Fprintf(w, " error=%q", record.DecodeError)
		}
		if record.Legacy != "" {
			_, _ = fmt.Fprintf(w, " legacy=%s", record.Legacy)
		}
		if record.KeyStatus != "" {
			_, _ = fmt.Fprintf(w, " key_status=%q", record.KeyStatus)
		}
		_, _ = fmt.Fprintln(w)
	}

	_, _ = fmt.Fprintf(
		w, "summary total=%d v1_ok=%d v2_candidates=%d fatal=%d "+
			"exact_unsupported_format_3d=%d\n", report.Total,
		report.V1OK, report.V2Candidates, report.Fatal,
		report.ExactMatch,
	)

	_, _ = fmt.Fprintf(w, "verdict=%s\n", report.Verdict)
}

func waitingProofVerdict(report *waitingProofReport) string {
	switch {
	case report.BucketState == "absent" || report.BucketState == "empty":
		return "waiting_proof_store_ruled_out"

	case report.ExactMatch > 0:
		return "exact_reported_crash_reproduced"

	case report.Fatal > 0:
		return "other_startup_fatal_records_found"

	case report.V2Candidates > 0:
		return "needs_full_v2_decode"

	default:
		return "reported_crash_not_found"
	}
}
