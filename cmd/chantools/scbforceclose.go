package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"strings"

	"github.com/btcsuite/btcd/wire"
	"github.com/lightninglabs/chantools/btc"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightninglabs/chantools/scbforceclose"
	"github.com/lightningnetwork/lnd/chanbackup"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/lnwallet"
	"github.com/spf13/cobra"
)

type scbForceCloseCommand struct {
	APIURL  string
	Publish bool

	// channel.backup.
	SingleBackup string
	SingleFile   string
	MultiBackup  string
	MultiFile    string

	rootKey *rootKey
	cmd     *cobra.Command
}

func newScbForceCloseCommand() *cobra.Command {
	cc := &scbForceCloseCommand{}
	cc.cmd = &cobra.Command{
		Use: "scbforceclose",
		Short: "Force-close the last state that is in the SCB " +
			"provided",
		Long:    forceCloseWarning,
		Example: `chantools scbforceclose --multi_file channel.backup`,
		RunE:    cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.APIURL, "apiurl", defaultAPIURL, "API URL to use (must "+
			"be esplora compatible)",
	)

	cc.cmd.Flags().StringVar(
		&cc.SingleBackup, "single_backup", "", "a hex encoded single "+
			"channel backup obtained from exportchanbackup for "+
			"force-closing channels",
	)
	cc.cmd.Flags().StringVar(
		&cc.MultiBackup, "multi_backup", "", "a hex encoded "+
			"multi-channel backup obtained from exportchanbackup "+
			"for force-closing channels",
	)
	cc.cmd.Flags().StringVar(
		&cc.SingleFile, "single_file", "", "the path to a "+
			"single-channel backup file",
	)
	cc.cmd.Flags().StringVar(
		&cc.MultiFile, "multi_file", "", "the path to a "+
			"single-channel backup file (channel.backup)",
	)

	cc.cmd.Flags().BoolVar(
		&cc.Publish, "publish", false, "publish force-closing TX to "+
			"the chain API instead of just printing the TX",
	)

	cc.rootKey = newRootKey(cc.cmd, "decrypting the backup and signing tx")

	return cc.cmd
}

func (c *scbForceCloseCommand) Execute(_ *cobra.Command, _ []string) error {
	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %w", err)
	}

	api := &btc.ExplorerAPI{BaseURL: c.APIURL}

	keyRing := &lnd.HDKeyRing{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}

	signer := &lnd.Signer{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}
	signer.MusigSessionManager = input.NewMusigSessionManager(
		signer.FetchPrivateKey,
	)

	var backups []chanbackup.Single
	if c.SingleBackup != "" || c.SingleFile != "" {
		if c.SingleBackup != "" && c.SingleFile != "" {
			return errors.New("must not pass --single_backup and " +
				"--single_file together")
		}
		var singleBackupBytes []byte
		if c.SingleBackup != "" {
			singleBackupBytes, err = hex.DecodeString(
				c.SingleBackup,
			)
		} else if c.SingleFile != "" {
			singleBackupBytes, err = os.ReadFile(c.SingleFile)
		}
		if err != nil {
			return fmt.Errorf("failed to get single backup: %w",
				err)
		}
		var s chanbackup.Single
		r := bytes.NewReader(singleBackupBytes)
		if err := s.UnpackFromReader(r, keyRing); err != nil {
			return fmt.Errorf("failed to unpack single backup: %w",
				err)
		}
		backups = append(backups, s)
	}
	if c.MultiBackup != "" || c.MultiFile != "" {
		if len(backups) != 0 {
			return errors.New("must not pass single and multi " +
				"backups together")
		}
		if c.MultiBackup != "" && c.MultiFile != "" {
			return errors.New("must not pass --multi_backup and " +
				"--multi_file together")
		}
		var multiBackupBytes []byte
		if c.MultiBackup != "" {
			multiBackupBytes, err = hex.DecodeString(c.MultiBackup)
		} else if c.MultiFile != "" {
			multiBackupBytes, err = os.ReadFile(c.MultiFile)
		}
		if err != nil {
			return fmt.Errorf("failed to get multi backup: %w", err)
		}
		var m chanbackup.Multi
		r := bytes.NewReader(multiBackupBytes)
		if err := m.UnpackFromReader(r, keyRing); err != nil {
			return fmt.Errorf("failed to unpack multi backup: %w",
				err)
		}
		backups = append(backups, m.StaticBackups...)
	}

	backupsWithInputs := make([]chanbackup.Single, 0, len(backups))
	for _, s := range backups {
		if s.CloseTxInputs.IsSome() {
			backupsWithInputs = append(backupsWithInputs, s)
		}
	}

	fmt.Println()
	fmt.Printf("Found %d channel backups, %d of them have close tx.\n",
		len(backups), len(backupsWithInputs))

	if len(backupsWithInputs) == 0 {
		fmt.Println("No channel backups that can be used for force " +
			"close.")
		return nil
	}

	fmt.Println()
	fmt.Println("@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@")
	fmt.Println(strings.TrimSpace(forceCloseWarning))
	fmt.Println("@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@")
	fmt.Println()

	fmt.Printf("Type YES to proceed: ")
	var userInput string
	if _, err := fmt.Scan(&userInput); err != nil {
		return errors.New("failed to read user input")
	}
	if strings.TrimSpace(userInput) != "YES" {
		return errors.New("canceled by user, must type uppercase 'YES'")
	}

	if c.Publish {
		fmt.Println("Signed transactions will be broadcasted " +
			"automatically.")
		fmt.Printf("Type YES again to proceed: ")
		if _, err := fmt.Scan(&userInput); err != nil {
			return errors.New("failed to read user input")
		}
		if strings.TrimSpace(userInput) != "YES" {
			return errors.New("canceled by user, must type " +
				"uppercase 'YES'")
		}
	}

	for _, s := range backupsWithInputs {
		signedTx, err := scbforceclose.SignCloseTx(
			s, keyRing, signer, signer,
		)
		if err != nil {
			return fmt.Errorf("signCloseTx failed for %s: %w",
				s.FundingOutpoint, err)
		}
		var buf bytes.Buffer
		if err := signedTx.Serialize(&buf); err != nil {
			return fmt.Errorf("failed to serialize signed %s: %w",
				s.FundingOutpoint, err)
		}
		txHex := hex.EncodeToString(buf.Bytes())
		fmt.Println("Channel point:", s.FundingOutpoint)
		fmt.Println("Raw transaction hex:", txHex)

		// Classify outputs: identify to_remote using known templates,
		// anchors (330 sat), and log the rest as to_local/htlc without
		// deriving per-commitment. The remote key is not tweaked in
		// the backup (except for very old channels which we don't
		// support anyway).
		classifyAndLogOutputs(s, signedTx)
		fmt.Println()

		// Publish TX.
		if c.Publish {
			response, err := api.PublishTx(txHex)
			if err != nil {
				return err
			}
			log.Infof("Published TX %s, response: %s",
				signedTx.TxHash(), response)
		}
	}

	return nil
}

// classifyAndLogOutputs attempts to identify the to_remote output by comparing
// against known script templates (p2wkh, delayed p2wsh, lease), marks 330-sat
// anchors, and logs remaining outputs as to_local/htlc without deriving
// per-commitment data.
func classifyAndLogOutputs(s chanbackup.Single, tx *wire.MsgTx) {
	class := classifyOutputs(s, tx)
	if class.ToRemoteIdx >= 0 {
		log.Infof(
			"to_remote: idx=%d amount=%d sat", class.ToRemoteIdx,
			class.ToRemoteAmt,
		)
		if len(class.ToRemotePkScript) != 0 {
			log.Infof(
				"to_remote PkScript (hex): %x",
				class.ToRemotePkScript,
			)
		}
	} else {
		log.Infof("to_remote: not identified")
	}
	for _, idx := range class.AnchorIdxs {
		log.Infof(
			"possible anchor: idx=%d amount=%d sat", idx,
			tx.TxOut[idx].Value,
		)
	}
	for _, idx := range class.OtherIdxs {
		log.Infof(
			"possible to_local/htlc: idx=%d amount=%d sat", idx,
			tx.TxOut[idx].Value,
		)
	}
}

type outputClassification struct {
	ToRemoteIdx      int
	ToRemoteAmt      int64
	ToRemotePkScript []byte
	AnchorIdxs       []int
	OtherIdxs        []int
}

func classifyOutputs(s chanbackup.Single, tx *wire.MsgTx) outputClassification {
	// Best-effort get the remote key used for to_remote.
	remoteDesc := s.RemoteChanCfg.PaymentBasePoint
	remoteKey := remoteDesc.PubKey

	// Compute the expected to_remote pkScript.
	var toRemotePkScript []byte
	if remoteKey != nil {
		chanType, err := chanTypeFromBackupVersion(s.Version)
		if err == nil {
			desc, _, err := lnwallet.CommitScriptToRemote(
				chanType, s.IsInitiator, remoteKey,
				s.LeaseExpiry,
				input.NoneTapLeaf(),
			)
			if err == nil {
				toRemotePkScript = desc.PkScript()
			}
		}
	}

	const anchorVSize = int64(330) // anchor output value in sats

	result := outputClassification{
		ToRemoteIdx:      -1,
		ToRemotePkScript: toRemotePkScript,
	}

	// First pass: find to_remote by script match.
	for idx, out := range tx.TxOut {
		if len(toRemotePkScript) != 0 &&
			bytes.Equal(out.PkScript, toRemotePkScript) {

			result.ToRemoteIdx = idx
			result.ToRemoteAmt = out.Value
			break
		}
	}

	// Second pass: classify anchors and the rest.
	for idx, out := range tx.TxOut {
		if idx == result.ToRemoteIdx {
			continue
		}
		if out.Value == anchorVSize {
			result.AnchorIdxs = append(result.AnchorIdxs, idx)
		} else {
			result.OtherIdxs = append(result.OtherIdxs, idx)
		}
	}

	return result
}

// chanTypeFromBackupVersion maps a backup SingleBackupVersion to an approximate
// channeldb.ChannelType sufficient for deriving to_remote scripts.
func chanTypeFromBackupVersion(v chanbackup.SingleBackupVersion) (
	channeldb.ChannelType, error) {

	var chanType channeldb.ChannelType
	switch v {
	case chanbackup.DefaultSingleVersion:
		chanType = channeldb.SingleFunderBit

	case chanbackup.TweaklessCommitVersion:
		chanType = channeldb.SingleFunderTweaklessBit

	case chanbackup.AnchorsCommitVersion:
		chanType = channeldb.AnchorOutputsBit
		chanType |= channeldb.SingleFunderTweaklessBit

	case chanbackup.AnchorsZeroFeeHtlcTxCommitVersion:
		chanType = channeldb.ZeroHtlcTxFeeBit
		chanType |= channeldb.AnchorOutputsBit
		chanType |= channeldb.SingleFunderTweaklessBit

	case chanbackup.ScriptEnforcedLeaseVersion:
		chanType = channeldb.LeaseExpirationBit
		chanType |= channeldb.ZeroHtlcTxFeeBit
		chanType |= channeldb.AnchorOutputsBit
		chanType |= channeldb.SingleFunderTweaklessBit

	case chanbackup.SimpleTaprootVersion:
		chanType = channeldb.ZeroHtlcTxFeeBit
		chanType |= channeldb.AnchorOutputsBit
		chanType |= channeldb.SingleFunderTweaklessBit
		chanType |= channeldb.SimpleTaprootFeatureBit

	case chanbackup.TapscriptRootVersion:
		chanType = channeldb.ZeroHtlcTxFeeBit
		chanType |= channeldb.AnchorOutputsBit
		chanType |= channeldb.SingleFunderTweaklessBit
		chanType |= channeldb.SimpleTaprootFeatureBit
		chanType |= channeldb.TapscriptRootBit

	default:
		return 0, fmt.Errorf("unknown Single version: %v", v)
	}

	return chanType, nil
}
