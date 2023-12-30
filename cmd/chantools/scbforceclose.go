package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"os"
	"strings"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightninglabs/chantools/btc"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/chanbackup"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/lnwallet"
	"github.com/lightningnetwork/lnd/shachain"
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
		Use:     "scbforceclose",
		Short:   "Force-close the last state that is in the SCB provided",
		Long:    forceCloseWarning,
		Example: `chantools scbforceclose --multi_file channel.backup`,
		RunE:    cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.APIURL, "apiurl", defaultAPIURL, "API URL to use (must "+
			"be esplora compatible)",
	)

	cc.cmd.Flags().StringVar(
		&cc.SingleBackup, "single_backup", "", "a hex encoded single channel "+
			"backup obtained from exportchanbackup for force-closing channels",
	)
	cc.cmd.Flags().StringVar(
		&cc.MultiBackup, "multi_backup", "", "a hex encoded multi-channel "+
			"backup obtained from exportchanbackup for force-closing channels",
	)
	cc.cmd.Flags().StringVar(
		&cc.SingleFile, "single_file", "", "the path to a single-channel "+
			"backup file",
	)
	cc.cmd.Flags().StringVar(
		&cc.MultiFile, "multi_file", "", "the path to a single-channel "+
			"backup file (channel.backup)",
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
	var backups []chanbackup.Single
	if c.SingleBackup != "" || c.SingleFile != "" {
		if c.SingleBackup != "" && c.SingleFile != "" {
			return fmt.Errorf("must not pass --single_backup and " +
				"--single_file together")
		}
		var singleBackupBytes []byte
		if c.SingleBackup != "" {
			singleBackupBytes, err = hex.DecodeString(c.SingleBackup)
		} else if c.SingleFile != "" {
			singleBackupBytes, err = os.ReadFile(c.SingleFile)
		}
		if err != nil {
			return fmt.Errorf("failed to get single backup: %w", err)
		}
		var s chanbackup.Single
		r := bytes.NewReader(singleBackupBytes)
		if err := s.UnpackFromReader(r, keyRing); err != nil {
			return fmt.Errorf("failed to unpack single backup: %w", err)
		}
		backups = append(backups, s)
	}
	if c.MultiBackup != "" || c.MultiFile != "" {
		if len(backups) != 0 {
			return fmt.Errorf("must not pass single and multi " +
				"backups together")
		}
		if c.MultiBackup != "" && c.MultiFile != "" {
			return fmt.Errorf("must not pass --multi_backup and " +
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
			return fmt.Errorf("failed to unpack multi backup: %w", err)
		}
		backups = append(backups, m.StaticBackups...)
	}

	backupsWithInputs := make([]chanbackup.Single, 0, len(backups))
	for _, s := range backups {
		if s.CloseTxInputs != nil {
			backupsWithInputs = append(backupsWithInputs, s)
		}
	}

	fmt.Println()
	fmt.Printf("Found %d channel backups, %d of them have close tx.\n",
		len(backups), len(backupsWithInputs))

	if len(backupsWithInputs) == 0 {
		fmt.Println("No channel backups that can be used for force close.")
		return nil
	}

	fmt.Println()
	fmt.Println("@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@")
	fmt.Println(strings.TrimSpace(forceCloseWarning))
	fmt.Println("@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@@")
	fmt.Println()

	fmt.Printf("Type YES to proceed: ")
	var userInput string
	fmt.Scan(&userInput)
	if strings.TrimSpace(userInput) != "YES" {
		return fmt.Errorf("cancelled by user")
	}

	if c.Publish {
		fmt.Println("Signed transactions will be broadcasted automatically.")
		fmt.Printf("Type YES again to proceed: ")
		fmt.Scan(&userInput)
		if strings.TrimSpace(userInput) != "YES" {
			return fmt.Errorf("cancelled by user")
		}
	}

	for _, s := range backupsWithInputs {
		signedTx, err := signCloseTx(s, extendedKey)
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
		fmt.Println(s.FundingOutpoint)
		fmt.Println(txHex)
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

func signCloseTx(s chanbackup.Single, extendedKey *hdkeychain.ExtendedKey) (
	*wire.MsgTx, error) {

	if s.CloseTxInputs == nil {
		return nil, fmt.Errorf("channel backup does not have data needed " +
			"to sign force sloe tx")
	}

	// Each of the keys in our local channel config only have their
	// locators populate, so we'll re-derive the raw key now.
	keyRing := &lnd.HDKeyRing{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}
	var err error
	s.LocalChanCfg.MultiSigKey, err = keyRing.DeriveKey(
		s.LocalChanCfg.MultiSigKey.KeyLocator,
	)
	if err != nil {
		return nil, fmt.Errorf("unable to derive multi sig key: %w", err)
	}

	signDesc, err := createSignDesc(s)
	if err != nil {
		return nil, fmt.Errorf("failed to create signDesc: %w", err)
	}

	inputs := lnwallet.SignedCommitTxInputs{
		CommitTx:  s.CloseTxInputs.CommitTx,
		CommitSig: s.CloseTxInputs.CommitSig,
		OurKey:    s.LocalChanCfg.MultiSigKey,
		TheirKey:  s.RemoteChanCfg.MultiSigKey,
		SignDesc:  signDesc,
	}

	if s.Version == chanbackup.SimpleTaprootVersion {
		p, err := createTaprootNonceProducer(s, extendedKey)
		if err != nil {
			return nil, err
		}
		inputs.Taproot = &lnwallet.TaprootSignedCommitTxInputs{
			CommitHeight:         s.CloseTxInputs.CommitHeight,
			TaprootNonceProducer: p,
		}
	}

	signer := &lnd.Signer{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}
	musigSessionManager := input.NewMusigSessionManager(signer.FetchPrivKey)
	signer.MusigSessionManager = musigSessionManager

	return lnwallet.GetSignedCommitTx(inputs, signer)
}

func createSignDesc(s chanbackup.Single) (*input.SignDescriptor, error) {
	// See LightningChannel.createSignDesc on how signDesc is produced.

	var fundingPkScript, multiSigScript []byte

	localKey := s.LocalChanCfg.MultiSigKey.PubKey
	remoteKey := s.RemoteChanCfg.MultiSigKey.PubKey

	var err error
	if s.Version == chanbackup.SimpleTaprootVersion {
		fundingPkScript, _, err = input.GenTaprootFundingScript(
			localKey, remoteKey, int64(s.Capacity),
		)
		if err != nil {
			return nil, err
		}
	} else {
		multiSigScript, err = input.GenMultiSigScript(
			localKey.SerializeCompressed(),
			remoteKey.SerializeCompressed(),
		)
		if err != nil {
			return nil, err
		}

		fundingPkScript, err = input.WitnessScriptHash(multiSigScript)
		if err != nil {
			return nil, err
		}
	}

	return &input.SignDescriptor{
		KeyDesc:       s.LocalChanCfg.MultiSigKey,
		WitnessScript: multiSigScript,
		Output: &wire.TxOut{
			PkScript: fundingPkScript,
			Value:    int64(s.Capacity),
		},
		HashType: txscript.SigHashAll,
		PrevOutputFetcher: txscript.NewCannedPrevOutputFetcher(
			fundingPkScript, int64(s.Capacity),
		),
		InputIndex: 0,
	}, nil
}

func createTaprootNonceProducer(
	s chanbackup.Single,
	extendedKey *hdkeychain.ExtendedKey,
) (shachain.Producer, error) {

	revPathStr := fmt.Sprintf("m/1017'/%d'/%d'/0/%d",
		chainParams.HDCoinType,
		s.ShaChainRootDesc.KeyLocator.Family,
		s.ShaChainRootDesc.KeyLocator.Index,
	)
	revPath, err := lnd.ParsePath(revPathStr)
	if err != nil {
		return nil, err
	}

	if s.ShaChainRootDesc.PubKey != nil {
		return nil, fmt.Errorf("taproot channels always use ECDH, " +
			"but legacy ShaChainRootDesc with pubkey found")
	}
	revocationProducer, err := lnd.ShaChainFromPath(
		extendedKey, revPath, s.LocalChanCfg.MultiSigKey.PubKey,
	)
	if err != nil {
		return nil, fmt.Errorf("lnd.ShaChainFromPath(extendedKey, %v, %v) "+
			"failed: %w", revPath, s.ShaChainRootDesc.PubKey, err)
	}

	return channeldb.DeriveMusig2Shachain(revocationProducer)
}
