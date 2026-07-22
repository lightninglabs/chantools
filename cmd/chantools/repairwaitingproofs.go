package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/spf13/cobra"
	bbolt "go.etcd.io/bbolt"
)

type repairWaitingProofsCommand struct {
	ChannelDB string
	Commit    bool

	cmd *cobra.Command
}

func newRepairWaitingProofsCommand() *cobra.Command {
	cc := &repairWaitingProofsCommand{}
	cc.cmd = &cobra.Command{
		Use: "repairwaitingproofs",
		Short: "Repair legacy lnd waiting proofs that can prevent " +
			"startup",
		Long: `This command repairs the specific waitingproofs bucket state that
can prevent affected lnd v0.21 nodes from starting with errors such as:

  invalid public key: unsupported format: 3d

The command only migrates clean legacy V1 waiting proof records from the old
format to the typed V1 format expected by lnd v0.21. It does not write or
invent metadata/dbp, and it refuses to overwrite conflicting typed records.

By default this command performs a dry run. To modify the database, stop lnd,
make sure you are operating on the intended channel.db, and pass --commit. When
--commit is used, the command creates a timestamped backup next to channel.db
before writing any changes.`,
		Example: `chantools repairwaitingproofs \
	--channeldb ~/.lnd/data/graph/mainnet/channel.db

chantools repairwaitingproofs \
	--channeldb ~/.lnd/data/graph/mainnet/channel.db \
	--commit`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.ChannelDB, "channeldb", "", "offline lnd channel.db "+
			"file to repair",
	)
	cc.cmd.Flags().BoolVar(
		&cc.Commit, "commit", false, "modify channel.db after "+
			"creating a backup; without this flag only a dry run is "+
			"performed",
	)

	return cc.cmd
}

func (c *repairWaitingProofsCommand) Execute(cmd *cobra.Command,
	_ []string) error {

	if c.ChannelDB == "" {
		return errors.New("channel DB is required")
	}

	report, err := repairWaitingProofDB(c.ChannelDB, c.Commit)
	if err != nil {
		return err
	}

	writeRepairWaitingProofReport(cmd.OutOrStdout(), report)

	return nil
}

type repairWaitingProofReport struct {
	DryRun              bool
	BackupPath          string
	DBVersionStatus     string
	BucketState         string
	TotalRecords        int
	LegacyClean         int
	TypedRecords        int
	OtherRecords        int
	Migrated            int
	WouldMigrate        int
	DeletedLegacy       int
	ConflictingTypedKey string
}

type waitingProofRepairAction struct {
	oldKey   []byte
	newKey   []byte
	newValue []byte
}

func repairWaitingProofDB(path string, commit bool) (*repairWaitingProofReport,
	error) {

	if !commit {
		return dryRunRepairWaitingProofDB(path)
	}

	db, err := bbolt.Open(path, dbFilePermission, &bbolt.Options{
		Timeout: 5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("error opening channel DB: %w", err)
	}
	defer func() { _ = db.Close() }()

	backupPath, err := backupFile(path)
	if err != nil {
		return nil, err
	}

	report := &repairWaitingProofReport{
		DryRun:     false,
		BackupPath: backupPath,
	}

	err = db.Update(func(tx *bbolt.Tx) error {
		return collectAndApplyWaitingProofRepair(tx, report, true)
	})
	if err != nil {
		return nil, fmt.Errorf("error repairing waiting proofs: %w", err)
	}

	return report, nil
}

func dryRunRepairWaitingProofDB(path string) (*repairWaitingProofReport, error) {
	db, err := bbolt.Open(path, dbFilePermission, &bbolt.Options{
		ReadOnly: true,
		Timeout:  5 * time.Second,
	})
	if err != nil {
		return nil, fmt.Errorf("error opening channel DB read-only: %w",
			err)
	}
	defer func() { _ = db.Close() }()

	report := &repairWaitingProofReport{
		DryRun: true,
	}

	err = db.View(func(tx *bbolt.Tx) error {
		return collectAndApplyWaitingProofRepair(tx, report, false)
	})
	if err != nil {
		return nil, fmt.Errorf("error inspecting waiting proofs: %w", err)
	}

	return report, nil
}

type waitingProofTx interface {
	Bucket(name []byte) *bbolt.Bucket
}

func collectAndApplyWaitingProofRepair(tx waitingProofTx,
	report *repairWaitingProofReport, apply bool) error {

	report.DBVersionStatus = dbVersionStatus(tx)
	report.BucketState = "absent"

	bucket := tx.Bucket(waitingProofsBucket)
	if bucket == nil {
		return nil
	}

	report.BucketState = "empty"

	var actions []waitingProofRepairAction
	err := bucket.ForEach(func(k, v []byte) error {
		if v == nil {
			return nil
		}

		report.BucketState = "present"
		report.TotalRecords++

		record := classifyWaitingProof(k, v)
		switch {
		case record.Legacy == "clean_legacy_v1":
			report.LegacyClean++
			action := legacyWaitingProofRepairAction(k, v)
			existing := bucket.Get(action.newKey)
			switch {
			case existing == nil:

			case bytes.Equal(existing, action.newValue):
				// The typed record is already present. We can safely
				// remove the duplicate legacy key during commit.

			default:
				report.ConflictingTypedKey = hex.EncodeToString(
					action.newKey,
				)
				return fmt.Errorf("typed waiting proof key %x "+
					"already exists with different value",
					action.newKey)
			}

			actions = append(actions, action)

		case len(k) == typedWaitingProofKeyLen:
			report.TypedRecords++

		default:
			report.OtherRecords++
		}

		return nil
	})
	if err != nil {
		return err
	}

	report.WouldMigrate = len(actions)
	if !apply {
		return nil
	}

	for _, action := range actions {
		if existing := bucket.Get(action.newKey); existing == nil {
			err := bucket.Put(action.newKey, action.newValue)
			if err != nil {
				return err
			}
			report.Migrated++
		}

		if err := bucket.Delete(action.oldKey); err != nil {
			return err
		}
		report.DeletedLegacy++
	}

	return nil
}

func legacyWaitingProofRepairAction(k, v []byte) waitingProofRepairAction {
	newKey := make([]byte, typedWaitingProofKeyLen)
	newKey[0] = waitingProofV1Type
	copy(newKey[1:9], k[:8])
	newKey[9] = k[8]

	newValue := make([]byte, len(v)+1)
	newValue[0] = waitingProofV1Type
	copy(newValue[1:], v)

	return waitingProofRepairAction{
		oldKey:   append([]byte(nil), k...),
		newKey:   newKey,
		newValue: newValue,
	}
}

func dbVersionStatus(tx waitingProofTx) string {
	meta := tx.Bucket(metadataBucket)
	if meta == nil {
		return "metadata_bucket_missing"
	}

	versionBytes := meta.Get(dbVersionKey)
	switch {
	case versionBytes == nil:
		return "db_version_key_missing"

	case len(versionBytes) == 4:
		return "present"

	default:
		return fmt.Sprintf(
			"db_version_key_malformed_len_%d", len(versionBytes),
		)
	}
}

func backupFile(path string) (string, error) {
	backupPath := fmt.Sprintf(
		"%s.repairwaitingproofs.%s.bak", path,
		time.Now().UTC().Format("20060102-150405.000000000"),
	)

	source, err := os.Open(path)
	if err != nil {
		return "", fmt.Errorf("error opening channel DB for backup: %w",
			err)
	}
	defer func() { _ = source.Close() }()

	dest, err := os.OpenFile(
		backupPath, os.O_WRONLY|os.O_CREATE|os.O_EXCL, dbFilePermission,
	)
	if err != nil {
		return "", fmt.Errorf("error creating backup %s: %w",
			backupPath, err)
	}
	defer func() { _ = dest.Close() }()

	if _, err := io.Copy(dest, source); err != nil {
		return "", fmt.Errorf("error writing backup %s: %w", backupPath,
			err)
	}
	if err := dest.Sync(); err != nil {
		return "", fmt.Errorf("error syncing backup %s: %w", backupPath,
			err)
	}

	return backupPath, nil
}

func writeRepairWaitingProofReport(w io.Writer,
	report *repairWaitingProofReport) {

	_, _ = fmt.Fprintf(w, "dry_run=%v\n", report.DryRun)
	if report.BackupPath != "" {
		_, _ = fmt.Fprintf(w, "backup=%s\n", report.BackupPath)
	}

	_, _ = fmt.Fprintf(w, "db_version_status=%s\n", report.DBVersionStatus)
	_, _ = fmt.Fprintf(w, "waitingproofs_bucket=%s\n", report.BucketState)
	_, _ = fmt.Fprintf(
		w, "summary total=%d clean_legacy=%d typed=%d other=%d "+
			"would_migrate=%d migrated=%d deleted_legacy=%d\n",
		report.TotalRecords, report.LegacyClean, report.TypedRecords,
		report.OtherRecords, report.WouldMigrate, report.Migrated,
		report.DeletedLegacy,
	)

	switch {
	case report.ConflictingTypedKey != "":
		_, _ = fmt.Fprintf(
			w, "verdict=conflict typed_key=%s\n",
			report.ConflictingTypedKey,
		)

	case report.DryRun && report.WouldMigrate > 0:
		_, _ = fmt.Fprintln(w, "verdict=would_repair")

	case report.DryRun:
		_, _ = fmt.Fprintln(w, "verdict=nothing_to_repair")

	case report.Migrated > 0 || report.DeletedLegacy > 0:
		_, _ = fmt.Fprintln(w, "verdict=repaired")

	default:
		_, _ = fmt.Fprintln(w, "verdict=nothing_to_repair")
	}
}
