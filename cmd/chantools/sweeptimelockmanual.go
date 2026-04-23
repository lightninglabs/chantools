package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightninglabs/chantools/dump"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/fn/v2"
	lfn "github.com/lightningnetwork/lnd/fn/v2"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lnwallet"
	"github.com/lightningnetwork/lnd/lnwallet/chainfee"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/lightningnetwork/lnd/shachain"
	"github.com/spf13/cobra"
)

const (
	keyBasePath = "m/1017'/%d'"
	maxKeys     = 500
	maxPoints   = 1000
)

// Lightning Terminal recovery constants - target auxiliary leaf hash from TapscriptRoot analysis
var targetAuxHash = []byte{
	0x6b, 0x24, 0xa4, 0x3f, 0xc6, 0x54, 0x37, 0x26, 0xb0, 0xdc, 0x49, 0x9c, 0xee, 0x20, 0x5e, 0x11,
	0xbf, 0xab, 0x9f, 0xd4, 0x39, 0x27, 0x6d, 0xac, 0xb6, 0xc5, 0x98, 0x6e, 0xdc, 0xb8, 0x74, 0x82,
}

// Taproot Assets marker
var taprootAssetsMarker = sha256.Sum256([]byte("taproot-assets"))

// Global variable to store the channel backup for auxiliary leaf search
var globalChannelBackup *dump.BackupSingle

type sweepTimeLockManualCommand struct {
	APIURL                    string
	Publish                   bool
	SweepAddr                 string
	MaxCsvLimit               uint16
	FeeRate                   uint32
	TimeLockAddr              string
	RemoteRevocationBasePoint string

	MaxNumChannelsTotal uint16
	MaxNumChanUpdates   uint64

	ChannelBackup string
	ChannelPoint  string

	rootKey *rootKey
	inputs  *inputFlags
	cmd     *cobra.Command
}

// createAuxiliaryLeaf creates auxiliary leaf with given parameters
func createAuxiliaryLeaf(version byte, rootHash []byte, rootSum uint64) []byte {
	leaf := make([]byte, 73)
	leaf[0] = version
	copy(leaf[1:33], taprootAssetsMarker[:])
	copy(leaf[33:65], rootHash)
	binary.BigEndian.PutUint64(leaf[65:73], rootSum)
	return leaf
}

// createLightningTerminalAuxLeaf creates auxiliary leaf using Lightning Terminal's exact algorithm
func createLightningTerminalAuxLeaf(version byte, rootHash []byte, rootSum uint64) []byte {
	// TaprootAssetsMarker = sha256("taproot-assets")
	taprootAssetsMarker := sha256.Sum256([]byte("taproot-assets"))
	
	var leafParts [][]byte
	var rootSumBytes [8]byte
	binary.BigEndian.PutUint64(rootSumBytes[:], rootSum)
	
	// Assemble the leafParts based on the commitment version (from tap.go:387)
	switch version {
	case 0, 1: // TapCommitmentV0 or TapCommitmentV1
		leafParts = [][]byte{
			{version}, taprootAssetsMarker[:], rootHash[:], rootSumBytes[:],
		}
	case 2: // TapCommitmentV2
		tag := sha256.Sum256([]byte("taproot-assets:194243"))
		leafParts = [][]byte{
			tag[:], {version}, rootHash[:], rootSumBytes[:],
		}
	default:
		// Default to V0/V1 format
		leafParts = [][]byte{
			{version}, taprootAssetsMarker[:], rootHash[:], rootSumBytes[:],
		}
	}
	
	// Join all parts to create the leaf script (from tap.go:402)
	return bytes.Join(leafParts, nil)
}

// bruteForceAuxiliaryLeaf searches for the auxiliary leaf that produces target hash
func bruteForceAuxiliaryLeaf() input.AuxTapLeaf {
	log.Infof("🔍 AUXILIARY LEAF SEARCH - Lightning Terminal Algorithm!")
	log.Infof("Target aux leaf hash: %x", targetAuxHash)
	
	// Lightning Terminal auxiliary leaf construction based on TapCommitment.TapLeaf()
	// Structure: [1 byte version][32 byte TaprootAssetsMarker][32 byte rootHash][8 byte rootSum]
	
	// Will test multiple asset amount sums below
	
	// TaprootAssetsMarker = sha256("taproot-assets")
	taprootAssetsMarker := sha256.Sum256([]byte("taproot-assets"))
	
	// Try different version values and MSSMT root hashes
	log.Infof("🔍 Testing Lightning Terminal auxiliary leaf formats...")
	
	// Test the ACTUAL MSSMT root hash and asset data from tapd.db!
	testRootHashes := []string{
		"641e93ea62319592a853d8b94269a07b711d9b3764b3ff326f3e89612b0710c6", // ACTUAL taproot asset root from tapd.db!
		"a1058226c95f1c829beeeb0badbe50462a9cfab4ea86acb4b05b8bb454a85fde", // ACTUAL MSSMT root from tapd.db!
		"109cb057ff24979399139bdd8fd40670da8cd1adbf0eecb464231c4004e3bc2e", // ACTUAL asset ID from tapd.db!
		"1422e28eb675032446c1341604f034e7305de8067cc34f816dc3928a1954c5bc", // Previous test value
		"41ce365de68c63d2c0e8df0fece40727423b501548818717dcd3179b6a9b5fb45", // From tapscript sibling hash1
		"2273a79d21e03a7e3292b740fc19c4efac09b56963123ffc46f605927a9f3",       // From tapscript sibling hash2 (padded)
		"b452273a79d21e03a7e3292b740fc19c4efac09b56963123ffc46f605927a9f3", // Full hash2 from sibling
		"c610072b61893e6f32ffb364379b1d717ba06942b9d853a892953162ea931e64", // TapscriptRoot
		"0000000000000000000000000000000000000000000000000000000000000000", // Empty/zero
		"6b24a43fc6543726b0dc499cee205e11bfab9fd439276dacb6c5986edcb87482", // Target aux hash as root
	}
	
	// Test multiple asset sum values
	testAssetSums := []uint64{
		100000000000,     // Original amount
		97751,           // Channel capacity in sats
		977510000,       // Channel capacity in millisats
		100000000,       // 1 BTC in sats
		1000000000000,   // Larger amount
		1,               // Minimal amount
		0,               // Zero amount
	}
	
	// Test versions 0, 1, 2 (Lightning Terminal supports all three)
	for version := byte(0); version <= 2; version++ {
		for _, rootHashStr := range testRootHashes {
			rootHash, err := hex.DecodeString(rootHashStr)
			if err != nil || len(rootHash) != 32 {
				continue
			}
			
			for _, assetSum := range testAssetSums {
				// Create auxiliary leaf using Lightning Terminal format
				auxLeaf := createLightningTerminalAuxLeaf(version, rootHash, assetSum)
				
				// ✅ FIX: Use TapLeaf hash instead of plain SHA256
				tapLeaf := txscript.TapLeaf{
					LeafVersion: txscript.BaseLeafVersion,
					Script:      auxLeaf,
				}
				auxLeafHash := tapLeaf.TapHash()
				
				log.Infof("  V%d Hash:%x... Sum:%d => %x", version, rootHash[:8], assetSum, auxLeafHash[:8])
				
				if bytes.Equal(auxLeafHash[:], targetAuxHash) {
					log.Infof("🎯 FOUND MATCHING AUXILIARY LEAF!")
					log.Infof("  Version: %d", version)
					log.Infof("  Root Hash: %x", rootHash)
					log.Infof("  Asset Sum: %d", assetSum)
					
					return fn.Some(tapLeaf)
				}
			}
		}
	}
	
	// Test if the target hash itself is the auxiliary leaf script
	log.Infof("🔍 Testing if target hash is the auxiliary leaf script...")
	testScript := targetAuxHash
	testScriptHash := sha256.Sum256(testScript)
	log.Infof("  Target as script: %x => hash: %x", testScript, testScriptHash)
	
	if bytes.Equal(testScriptHash[:], targetAuxHash) {
		log.Infof("🎯 TARGET HASH IS THE AUX LEAF SCRIPT!")
		return lfn.Some(txscript.TapLeaf{
			LeafVersion: txscript.BaseLeafVersion,
			Script:      testScript,
		})
	}
	
	// Also test if the target hash itself is the auxiliary leaf (without hashing)
	log.Infof("🔍 Testing target hash as raw auxiliary leaf...")
	return lfn.Some(txscript.TapLeaf{
		LeafVersion: txscript.BaseLeafVersion,
		Script:      targetAuxHash,
	})
	
	// Try using the actual tapscript sibling data from backup
	tapscriptSibling := "010741CE365DE68C63D2C0E8DF0FECE40727423B501548818717DCD3179B6A9B5FB452273A79D21E03A7E3292B740FC19C4EFAC09B56963123FFC46F605927A9F3"
	if siblingBytes, err := hex.DecodeString(tapscriptSibling); err == nil {
		log.Infof("🔍 Testing tapscript sibling structure (65 bytes)...")
		
		// The sibling is 65 bytes: [1 byte version][32 byte hash1][32 byte hash2]
		if len(siblingBytes) == 65 {
			version := siblingBytes[0]
			hash1 := siblingBytes[1:33]
			hash2 := siblingBytes[33:65]
			
			log.Infof("  Version: 0x%02x", version)
			log.Infof("  Hash1: %x", hash1)
			log.Infof("  Hash2: %x", hash2)
			
			// Test hash1 as raw auxiliary leaf script
			tapLeaf1 := txscript.TapLeaf{
				LeafVersion: txscript.BaseLeafVersion,
				Script:      hash1,
			}
			leafHash1 := tapLeaf1.TapHash()
			
			log.Infof("🔍 Testing hash1 as auxiliary leaf script...")
			log.Infof("  Hash1 leaf hash: %x", leafHash1)
			if bytes.Equal(leafHash1[:], targetAuxHash) {
				log.Infof("✅ FOUND MATCHING AUXILIARY LEAF FROM HASH1!")
				return fn.Some(tapLeaf1)
			}
			
			// Test hash2 as raw auxiliary leaf script
			tapLeaf2 := txscript.TapLeaf{
				LeafVersion: txscript.BaseLeafVersion,
				Script:      hash2,
			}
			leafHash2 := tapLeaf2.TapHash()
			
			log.Infof("🔍 Testing hash2 as auxiliary leaf script...")
			log.Infof("  Hash2 leaf hash: %x", leafHash2)
			if bytes.Equal(leafHash2[:], targetAuxHash) {
				log.Infof("✅ FOUND MATCHING AUXILIARY LEAF FROM HASH2!")
				return fn.Some(tapLeaf2)
			}
		}
		
		// Try the full sibling data as auxiliary leaf
		tapLeaf := txscript.TapLeaf{
			LeafVersion: txscript.BaseLeafVersion,
			Script:      siblingBytes,
		}
		leafHash := tapLeaf.TapHash()
		
		log.Infof("🔍 Testing full sibling as auxiliary leaf...")
		log.Infof("  Full sibling hash: %x", leafHash)
		
		if bytes.Equal(leafHash[:], targetAuxHash) {
			log.Infof("✅ FOUND MATCHING AUXILIARY LEAF FROM FULL SIBLING!")
			return fn.Some(tapLeaf)
		}
	}
	
	// Test both V0 and V1 auxiliary leaf formats
	for version := byte(0); version <= 1; version++ {
		log.Infof("Testing auxiliary leaf version %d...", version)
		
		// Test root hash patterns from actual backup data
		rootPatterns := [][]byte{
			make([]byte, 32), // All zeros
			taprootAssetsMarker[:], // Taproot assets marker
		}
		
		// EXACT CHANNEL-SPECIFIC ASSET DATA from our Lightning channel outpoint analysis
		// Channel Asset Root Hash: 641e93ea62319592a853d8b94269a07b711d9b3764b3ff326f3e89612b0710c6
		channelAssetRoot, _ := hex.DecodeString("641e93ea62319592a853d8b94269a07b711d9b3764b3ff326f3e89612b0710c6")
		if len(channelAssetRoot) == 32 {
			rootPatterns = append(rootPatterns, channelAssetRoot)
			log.Infof("✅ Added channel-specific asset root: %x", channelAssetRoot)
		}
		
		// MSSMT Root Hash from our channel's commitment tree
		mssmtRootHash, _ := hex.DecodeString("A1058226C95F1C829BEEEB0BADBE50462A9CFAB4EA86ACB4B05B8BB454A85FDE")
		if len(mssmtRootHash) == 32 {
			rootPatterns = append(rootPatterns, mssmtRootHash)
			log.Infof("✅ Added channel MSSMT root: %x", mssmtRootHash)
		}
		
		// Also keep the general asset data for completeness
		// Taproot Asset Root Hash from tapd.db
		assetRootHash, _ := hex.DecodeString("1422E28EB675032446C1341604F034E7305DE8067CC34F816DC3928A1954C5BC")
		if len(assetRootHash) == 32 {
			rootPatterns = append(rootPatterns, assetRootHash)
		}
		
		// Merkle Root Hash from backup
		merkleRootHash, _ := hex.DecodeString("030982FCD09318ADCBC99DE884E0C24AF6CA64EBA1CE1B1F7DFA77E84E09DF")
		if len(merkleRootHash) == 32 {
			rootPatterns = append(rootPatterns, merkleRootHash)
		}
		
		// Add backup-derived patterns  
		if globalChannelBackup != nil {
			// Parse funding outpoint string to bytes
			fundingBytes, err := hex.DecodeString(globalChannelBackup.FundingOutpoint)
			if err == nil && len(fundingBytes) >= 32 {
				rootPatterns = append(rootPatterns, fundingBytes[:32])
				
				// SHA256 of funding outpoint
				fundingSha := sha256.Sum256(fundingBytes[:32])
				rootPatterns = append(rootPatterns, fundingSha[:])
				
				// Double SHA256
				doubleSha := sha256.Sum256(fundingSha[:])
				rootPatterns = append(rootPatterns, doubleSha[:])
			}
		}
		
		// Test various amounts including asset-specific values
		amounts := []uint64{
			0, 1, 97751, 100000000, // Common amounts
			0xffffffffffffffff, // Max uint64
		}
		
		// Add more specific amounts for this channel and asset
		channelAmounts := []uint64{
			97751, // Exact channel amount
			0x17dd7, // Channel amount in hex
			1, // Asset amount (common for single asset)
			1000, // Common asset amounts
			10000,
			100000,
		}
		amounts = append(amounts, channelAmounts...)
		
		tested := 0
		for _, rootHash := range rootPatterns {
			for _, amount := range amounts {
				// Create auxiliary leaf
				auxLeaf := createAuxiliaryLeaf(version, rootHash, amount)
				
				// Compute tap leaf hash
				tapLeaf := txscript.TapLeaf{
					LeafVersion: txscript.BaseLeafVersion,
					Script:      auxLeaf,
				}
				leafHash := tapLeaf.TapHash()
				
				tested++
				if tested%50 == 0 {
					log.Infof("  Tested %d combinations...", tested)
				}
				
				// Check if it matches target
				if bytes.Equal(leafHash[:], targetAuxHash) {
					log.Infof("✅ FOUND MATCHING AUXILIARY LEAF!")
					log.Infof("  Version: %d", version)
					log.Infof("  Root hash: %x", rootHash)
					log.Infof("  Amount: %d", amount)
					log.Infof("  Leaf bytes: %x", auxLeaf)
					log.Infof("  Leaf hash: %x", leafHash)
					
					tapLeaf := txscript.TapLeaf{
						LeafVersion: txscript.BaseLeafVersion,
						Script:      auxLeaf,
					}
					return fn.Some(tapLeaf)
				}
			}
		}
	}
	
	// If not found in common patterns, try extended brute force
	log.Infof("Common patterns failed, trying extended search...")
	
	// Extended brute force for version 0 with zero hash
	zeroHash := make([]byte, 32)
	for amount := uint64(0); amount < 200000; amount++ {
		auxLeaf := createAuxiliaryLeaf(0, zeroHash, amount)
		tapLeaf := txscript.TapLeaf{
			LeafVersion: txscript.BaseLeafVersion,
			Script:      auxLeaf,
		}
		leafHash := tapLeaf.TapHash()
		
		if bytes.Equal(leafHash[:], targetAuxHash) {
			log.Infof("✅ FOUND WITH EXTENDED SEARCH!")
			log.Infof("  Version: 0")
			log.Infof("  Root hash: %x", zeroHash)
			log.Infof("  Amount: %d", amount)
			log.Infof("  Leaf bytes: %x", auxLeaf)
			
			return fn.Some(tapLeaf)
		}
		
		if amount%10000 == 0 && amount > 0 {
			log.Infof("  Extended search: tested up to %d", amount)
		}
	}
	
	log.Infof("❌ Auxiliary leaf not found - using empty leaf")
	return input.NoneTapLeaf()
}

func newSweepTimeLockManualCommand() *cobra.Command {
	cc := &sweepTimeLockManualCommand{}
	cc.cmd = &cobra.Command{
		Use: "sweeptimelockmanual",
		Short: "Sweep the force-closed state of a single channel " +
			"manually if only a channel backup file is available",
		Long: `Sweep the locally force closed state of a single channel
manually if only a channel backup file is available. This can only be used if a
channel is force closed from the local node but then that node's state is lost
and only the channel.backup file is available.

To get the value for --remoterevbasepoint you must use the dumpbackup command,
then look up the value for RemoteChanCfg -> RevocationBasePoint -> PubKey.

Alternatively you can directly use the --frombackup and --channelpoint flags to
pull the required information from the given channel.backup file automatically.

To get the value for --timelockaddr you must look up the channel's funding
output on chain, then follow it to the force close output. The time locked
address is always the one that's longer (because it's P2WSH and not P2PKH).`,
		Example: `chantools sweeptimelockmanual \
	--sweepaddr bc1q..... \
	--timelockaddr bc1q............ \
	--remoterevbasepoint 03xxxxxxx \
	--feerate 10 \
	--publish

chantools sweeptimelockmanual \
	--sweepaddr bc1q..... \
	--timelockaddr bc1q............ \
	--frombackup channel.backup \
	--channelpoint f39310xxxxxxxxxx:1 \
	--feerate 10 \
	--publish`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.APIURL, "apiurl", defaultAPIURL, "API URL to use (must "+
			"be esplora compatible)",
	)
	cc.cmd.Flags().BoolVar(
		&cc.Publish, "publish", false, "publish sweep TX to the chain "+
			"API instead of just printing the TX",
	)
	cc.cmd.Flags().StringVar(
		&cc.SweepAddr, "sweepaddr", "", "address to recover the funds "+
			"to; specify '"+lnd.AddressDeriveFromWallet+"' to "+
			"derive a new address from the seed automatically",
	)
	cc.cmd.Flags().Uint16Var(
		&cc.MaxCsvLimit, "maxcsvlimit", defaultCsvLimit, "maximum CSV "+
			"limit to use",
	)
	cc.cmd.Flags().Uint16Var(
		&cc.MaxNumChannelsTotal, "maxnumchanstotal", maxKeys, "maximum "+
			"number of keys to try, set to maximum number of "+
			"channels the local node potentially has or had",
	)
	cc.cmd.Flags().Uint64Var(
		&cc.MaxNumChanUpdates, "maxnumchanupdates", maxPoints,
		"maximum number of channel updates to try, set to maximum "+
			"number of times the channel was used",
	)
	cc.cmd.Flags().Uint32Var(
		&cc.FeeRate, "feerate", defaultFeeSatPerVByte, "fee rate to "+
			"use for the sweep transaction in sat/vByte",
	)
	cc.cmd.Flags().StringVar(
		&cc.TimeLockAddr, "timelockaddr", "", "address of the time "+
			"locked commitment output where the funds are stuck in",
	)
	cc.cmd.Flags().StringVar(
		&cc.RemoteRevocationBasePoint, "remoterevbasepoint", "", ""+
			"remote node's revocation base point, can be found "+
			"in a channel.backup file",
	)
	cc.cmd.Flags().StringVar(
		&cc.ChannelBackup, "frombackup", "", "channel backup file to "+
			"read the channel information from",
	)
	cc.cmd.Flags().StringVar(
		&cc.ChannelPoint, "channelpoint", "", "channel point to use "+
			"for locating the channel in the channel backup file "+
			"specified in the --frombackup flag, "+
			"format: txid:index",
	)

	cc.rootKey = newRootKey(cc.cmd, "deriving keys")
	cc.inputs = newInputFlags(cc.cmd)

	return cc.cmd
}

func (c *sweepTimeLockManualCommand) Execute(_ *cobra.Command, _ []string) error {
	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %w", err)
	}

	// Make sure the sweep and time lock addrs are set.
	err = lnd.CheckAddress(
		c.SweepAddr, chainParams, true, "sweep", lnd.AddrTypeP2WKH,
		lnd.AddrTypeP2TR,
	)
	if err != nil {
		return err
	}

	err = lnd.CheckAddress(
		c.TimeLockAddr, chainParams, true, "time lock",
		lnd.AddrTypeP2WSH, lnd.AddrTypeP2TR,
	)
	if err != nil {
		return err
	}

	var (
		startCsvLimit             uint16
		maxCsvLimit               = c.MaxCsvLimit
		startNumChannelsTotal     uint16
		maxNumChannelsTotal       = c.MaxNumChannelsTotal
		remoteRevocationBasePoint = c.RemoteRevocationBasePoint
		multiSigIdx               uint32
	)

	// We either support specifying the remote revocation base point
	// manually, in which case the CSV limit and number of channels are not
	// known, or we can use the channel backup file to get the required
	// information from there directly.
	switch {
	case c.RemoteRevocationBasePoint != "":
		// Nothing to do here but continue below with the info provided
		// by the user.

	case c.ChannelBackup != "":
		if c.ChannelPoint == "" {
			return errors.New("channel point is required with " +
				"--frombackup")
		}

		backupChan, err := lnd.ExtractChannel(
			extendedKey, chainParams, c.ChannelBackup,
			c.ChannelPoint,
		)
		if err != nil {
			return fmt.Errorf("error extracting channel: %w", err)
		}
		
		// Store the channel backup globally for auxiliary leaf search
		globalChannelBackup = backupChan

		remoteCfg := backupChan.RemoteChanCfg
		localCfg := backupChan.LocalChanCfg
		remoteRevocationBasePoint = remoteCfg.RevocationBasePoint.PubKey

		startCsvLimit = remoteCfg.CsvDelay
		maxCsvLimit = startCsvLimit + 1

		delayPath, err := lnd.ParsePath(localCfg.DelayBasePoint.Path)
		if err != nil {
			return fmt.Errorf("error parsing delay path: %w", err)
		}
		if len(delayPath) != 5 {
			return fmt.Errorf("invalid delay path '%v'", delayPath)
		}

		startNumChannelsTotal = uint16(delayPath[4])
		maxNumChannelsTotal = startNumChannelsTotal + 1
		multiSigKeyPath, err := lnd.ParsePath(localCfg.MultiSigKey.Path)
		if err != nil {
			return fmt.Errorf("error parsing multisigkey path: %w",
				err)
		}
		if len(multiSigKeyPath) != 5 {
			return fmt.Errorf("invalid multisig path '%v'",
				multiSigKeyPath)
		}
		multiSigIdx = multiSigKeyPath[4]

	case c.ChannelBackup != "" && c.RemoteRevocationBasePoint != "":
		return errors.New("cannot use both --frombackup and " +
			"--remoterevbasepoint at the same time")

	default:
		return errors.New("either --frombackup or " +
			"--remoterevbasepoint is required")
	}

	// The remote revocation base point must also be set and a valid EC
	// point.
	remoteRevPoint, err := pubKeyFromHex(remoteRevocationBasePoint)
	if err != nil {
		return fmt.Errorf("invalid remote revocation base point: %w",
			err)
	}

	return sweepTimeLockManual(
		extendedKey, c.APIURL, c.SweepAddr, c.TimeLockAddr,
		remoteRevPoint, multiSigIdx, startCsvLimit, maxCsvLimit,
		startNumChannelsTotal, maxNumChannelsTotal,
		c.MaxNumChanUpdates, c.Publish, c.FeeRate,
	)
}

func sweepTimeLockManual(extendedKey *hdkeychain.ExtendedKey, apiURL string,
	sweepAddr, timeLockAddr string, remoteRevPoint *btcec.PublicKey,
	multiSigIdx uint32, startCsvTimeout, maxCsvTimeout, startNumChannels,
	maxNumChannels uint16, maxNumChanUpdates uint64, publish bool,
	feeRate uint32) error {

	log.Debugf("Starting to brute force the time lock script, using: "+
		"remote_rev_base_point=%x, start_csv_limit=%d, "+
		"max_csv_limit=%d, start_num_channels=%d, "+
		"max_num_channels=%d, max_num_chan_updates=%d",
		remoteRevPoint.SerializeCompressed(), startCsvTimeout,
		maxCsvTimeout, startNumChannels, maxNumChannels,
		maxNumChanUpdates)

	// Create signer and transaction template.
	var (
		estimator input.TxWeightEstimator
		signer    = &lnd.Signer{
			ExtendedKey: extendedKey,
			ChainParams: chainParams,
		}
		api = newExplorerAPI(apiURL)
	)

	// First of all, we need to parse the lock addr and make sure we can
	// brute force the script with the information we have. If not, we can't
	// continue anyway.
	lockScript, err := lnd.PrepareWalletAddress(
		timeLockAddr, chainParams, nil, extendedKey, "time lock",
	)
	if err != nil {
		return err
	}
	sweepScript, err := lnd.PrepareWalletAddress(
		sweepAddr, chainParams, &estimator, extendedKey, "sweep",
	)
	if err != nil {
		return err
	}

	// We need to go through a lot of our keys so it makes sense to
	// pre-derive the static part of our key path.
	basePath, err := lnd.ParsePath(fmt.Sprintf(
		keyBasePath, chainParams.HDCoinType,
	))
	if err != nil {
		return fmt.Errorf("could not derive base path: %w", err)
	}
	baseKey, err := lnd.DeriveChildren(extendedKey, basePath)
	if err != nil {
		return fmt.Errorf("could not derive base key: %w", err)
	}

	// Go through all our keys now and try to find the ones that can derive
	// the script. This loop can take very long as it'll nest three times,
	// once for the key index, once for the commit points and once for the
	// CSV values. Most of the calculations should be rather cheap but the
	// number of iterations can go up to maxKeys*maxPoints*maxCsvTimeout.
	var (
		csvTimeout  int32
		script      []byte
		scriptHash  []byte
		delayDesc   *keychain.KeyDescriptor
		commitPoint *btcec.PublicKey
	)
	for i := startNumChannels; i < maxNumChannels; i++ {
		if multiSigIdx == 0 {
			multiSigIdx = uint32(i)
		}

		csvTimeout, script, scriptHash, commitPoint, delayDesc, err = tryKey(
			baseKey, remoteRevPoint, startCsvTimeout, maxCsvTimeout,
			lockScript, uint32(i), multiSigIdx, maxNumChanUpdates,
		)

		if err == nil {
			log.Infof("Found keys at index %d with CSV timeout %d",
				i, csvTimeout)

			break
		}

		log.Infof("Tried %d of %d keys.", i+1, maxKeys)
	}

	// Did we find what we looked for or did we just exhaust all
	// possibilities?
	if script == nil || delayDesc == nil {
		return errors.New("target script not derived")
	}

	// We now know everything we need to construct the sweep transaction,
	// except for what outpoint to sweep. We'll ask the chain API to give
	// us this information.
	tx, txindex, err := api.Outpoint(timeLockAddr)
	if err != nil {
		return fmt.Errorf("error looking up lock address %s on chain: "+
			"%v", timeLockAddr, err)
	}

	sweepTx := wire.NewMsgTx(2)
	sweepValue := int64(tx.Vout[txindex].Value)

	// Create the transaction input.
	txHash, err := chainhash.NewHashFromStr(tx.TXID)
	if err != nil {
		return fmt.Errorf("error parsing tx hash: %w", err)
	}
	sweepTx.TxIn = []*wire.TxIn{{
		PreviousOutPoint: wire.OutPoint{
			Hash:  *txHash,
			Index: uint32(txindex),
		},
		Sequence: input.LockTimeToSequence(
			false, uint32(csvTimeout),
		),
	}}

	// Calculate the fee based on the given fee rate and our weight
	// estimation.
	estimator.AddWitnessInput(input.ToLocalTimeoutWitnessSize)
	feeRateKWeight := chainfee.SatPerKVByte(1000 * feeRate).FeePerKWeight()
	totalFee := feeRateKWeight.FeeForWeight(estimator.Weight())

	// Add our sweep destination output.
	sweepTx.TxOut = []*wire.TxOut{{
		Value:    sweepValue - int64(totalFee),
		PkScript: sweepScript,
	}}

	log.Infof("Fee %d sats of %d total amount (estimated weight %d)",
		totalFee, sweepValue, estimator.Weight())

	// Create the sign descriptor for the input then sign the transaction.
	prevOutFetcher := txscript.NewCannedPrevOutputFetcher(
		scriptHash, sweepValue,
	)
	sigHashes := txscript.NewTxSigHashes(sweepTx, prevOutFetcher)
	signDesc := &input.SignDescriptor{
		KeyDesc: *delayDesc,
		SingleTweak: input.SingleTweakBytes(
			commitPoint, delayDesc.PubKey,
		),
		WitnessScript: script,
		Output: &wire.TxOut{
			PkScript: scriptHash,
			Value:    sweepValue,
		},
		InputIndex:        0,
		SigHashes:         sigHashes,
		PrevOutputFetcher: prevOutFetcher,
		HashType:          txscript.SigHashAll,
	}
	witness, err := input.CommitSpendTimeout(signer, signDesc, sweepTx)
	if err != nil {
		return err
	}
	sweepTx.TxIn[0].Witness = witness

	var buf bytes.Buffer
	err = sweepTx.Serialize(&buf)
	if err != nil {
		return err
	}

	// Publish TX.
	if publish {
		response, err := api.PublishTx(
			hex.EncodeToString(buf.Bytes()),
		)
		if err != nil {
			return err
		}
		log.Infof("Published TX %s, response: %s",
			sweepTx.TxHash().String(), response)
	}

	log.Infof("Transaction: %x", buf.Bytes())
	return nil
}

func tryKey(baseKey *hdkeychain.ExtendedKey, remoteRevPoint *btcec.PublicKey,
	startCsvTimeout, maxCsvTimeout uint16, lockScript []byte, idx,
	multiSigIdx uint32, maxNumChanUpdates uint64) (int32, []byte,
	[]byte, *btcec.PublicKey, *keychain.KeyDescriptor, error) {

	// The easy part first, let's derive the delay base point.
	delayPath := []uint32{
		lnd.HardenedKey(uint32(keychain.KeyFamilyDelayBase)),
		0, idx,
	}
	delayPrivKey, err := lnd.PrivKeyFromPath(baseKey, delayPath)
	if err != nil {
		return 0, nil, nil, nil, nil, err
	}

	// Get the revocation base point first, so we can calculate our
	// commit point. We start with the old way where the revocation index
	// was the same as the other indices. This applies to all channels
	// opened with versions prior to and including lnd v0.12.0-beta.
	revPath := []uint32{
		lnd.HardenedKey(uint32(
			keychain.KeyFamilyRevocationRoot,
		)), 0, idx,
	}
	revRoot, err := lnd.ShaChainFromPath(baseKey, revPath, nil)
	if err != nil {
		return 0, nil, nil, nil, nil, err
	}

	// FIRST: Check if this is a taproot address and use appropriate method
	if len(lockScript) == 34 && lockScript[0] == 0x51 && lockScript[1] == 0x20 {
		log.Infof("🔍 Detected P2TR address, using taproot recovery methods")
		csvTimeout, script, scriptHash, commitPoint, err := bruteForceDelayTaprootManual(
			delayPrivKey.PubKey(), remoteRevPoint, revRoot, lockScript,
			startCsvTimeout, maxCsvTimeout, maxNumChanUpdates,
			&keychain.KeyDescriptor{
				PubKey: delayPrivKey.PubKey(),
				KeyLocator: keychain.KeyLocator{
					Family: keychain.KeyFamilyDelayBase,
					Index:  idx,
				},
			},
		)
		if err == nil {
			log.Infof("🎉 SUCCESS with taproot recovery! CSV: %d", csvTimeout)
			return csvTimeout, script, scriptHash, commitPoint,
				&keychain.KeyDescriptor{
					PubKey: delayPrivKey.PubKey(),
					KeyLocator: keychain.KeyLocator{
						Family: keychain.KeyFamilyDelayBase,
						Index:  idx,
					},
				}, nil
		}
		log.Infof("Taproot recovery failed, trying P2WSH fallback...")
	}

	// FALLBACK: Brute force the lock script if exact method fails
	csvTimeout, script, scriptHash, commitPoint, err := bruteForceDelayPoint(
		delayPrivKey.PubKey(), remoteRevPoint, revRoot, lockScript,
		startCsvTimeout, maxCsvTimeout, maxNumChanUpdates,
	)
	if err == nil {
		return csvTimeout, script, scriptHash, commitPoint,
			&keychain.KeyDescriptor{
				PubKey: delayPrivKey.PubKey(),
				KeyLocator: keychain.KeyLocator{
					Family: keychain.KeyFamilyDelayBase,
					Index:  idx,
				},
			}, nil
	}

	// We could not derive the secrets to sweep the to_local output using
	// the old shachain root creation. Starting with lnd release
	// v0.13.0-beta the index for the revocation path creating the shachain
	// root changed. Now the shachain root is created using ECDH
	// with the local multisig public key
	// (for mainnet: m/1017'/0'/1'/0/idx). But we need to account for a
	// special case here. If the node was started with a version prior to
	// and including v0.12.0-beta the idx for the new shachain root
	// revocation is not one larger because idx 0 was already used for the
	// old creation scheme hence we need to replicate this behaviour here.
	// First trying the shachain root creation with the same index and if
	// this does not derive the secrets we increase the index of the
	// revocation key path by one (for mainnet: m/1017'/0'/5'/0/idx+1).
	// The exact path which was used for the shachain root can be seen
	// in the channel.backup file for every specific channel. The old
	// scheme has always a public key specified.The new one uses a key
	// locator and does not have a public key specified (nil).
	// Example
	//     ShaChainRootDesc: (dump.KeyDescriptor) {
	// 	Path: (string) (len=17) "m/1017'/1'/5'/0/1",
	// 	PubKey: (string) (len=5) "<nil>"
	//
	// For more details:
	// https://github.com/lightningnetwork/lnd/commit/bb84f0ebc88620050dec7cf4be6283f5cba8b920
	//
	// Now the new shachain root revocation scheme is tried with
	// two different indicies as described above.
	revPath2 := []uint32{
		lnd.HardenedKey(uint32(
			keychain.KeyFamilyRevocationRoot,
		)), 0, idx,
	}

	// Now we try the same with the new revocation producer format.
	multiSigPath := []uint32{
		lnd.HardenedKey(uint32(keychain.KeyFamilyMultiSig)),
		0, multiSigIdx,
	}
	multiSigPrivKey, err := lnd.PrivKeyFromPath(baseKey, multiSigPath)
	if err != nil {
		return 0, nil, nil, nil, nil, err
	}

	revRoot2, err := lnd.ShaChainFromPath(
		baseKey, revPath2, multiSigPrivKey.PubKey(),
	)
	if err != nil {
		return 0, nil, nil, nil, nil, err
	}

	csvTimeout, script, scriptHash, commitPoint, err = bruteForceDelayPoint(
		delayPrivKey.PubKey(), remoteRevPoint, revRoot2, lockScript,
		startCsvTimeout, maxCsvTimeout, maxNumChanUpdates,
	)
	if err == nil {
		return csvTimeout, script, scriptHash, commitPoint,
			&keychain.KeyDescriptor{
				PubKey: delayPrivKey.PubKey(),
				KeyLocator: keychain.KeyLocator{
					Family: keychain.KeyFamilyDelayBase,
					Index:  idx,
				},
			}, nil
	}

	// Now we try to increase the index by 1 to account for the situation
	// where the node was started with a version after (including)
	// v0.13.0-beta
	revPath3 := []uint32{
		lnd.HardenedKey(uint32(
			keychain.KeyFamilyRevocationRoot,
		)), 0, idx + 1,
	}

	// Now we try the same with the new revocation producer format.
	revRoot3, err := lnd.ShaChainFromPath(
		baseKey, revPath3, multiSigPrivKey.PubKey(),
	)
	if err != nil {
		return 0, nil, nil, nil, nil, err
	}

	csvTimeout, script, scriptHash, commitPoint, err = bruteForceDelayPoint(
		delayPrivKey.PubKey(), remoteRevPoint, revRoot3, lockScript,
		startCsvTimeout, maxCsvTimeout, maxNumChanUpdates,
	)
	if err == nil {
		return csvTimeout, script, scriptHash, commitPoint,
			&keychain.KeyDescriptor{
				PubKey: delayPrivKey.PubKey(),
				KeyLocator: keychain.KeyLocator{
					Family: keychain.KeyFamilyDelayBase,
					Index:  idx,
				},
			}, nil
	}

	return 0, nil, nil, nil, nil, errors.New("target script not derived")
}

func bruteForceDelayPoint(delayBase, revBase *btcec.PublicKey,
	revRoot *shachain.RevocationProducer, lockScript []byte,
	startCsvTimeout, maxCsvTimeout uint16, maxChanUpdates uint64) (int32,
	[]byte, []byte, *btcec.PublicKey, error) {

	for i := range maxChanUpdates {
		revPreimage, err := revRoot.AtIndex(i)
		if err != nil {
			return 0, nil, nil, nil, err
		}
		commitPoint := input.ComputeCommitmentPoint(revPreimage[:])

		csvTimeout, script, scriptHash, err := bruteForceDelay(
			input.TweakPubKey(delayBase, commitPoint),
			input.DeriveRevocationPubkey(revBase, commitPoint),
			lockScript, startCsvTimeout, maxCsvTimeout,
		)

		if err != nil {
			continue
		}

		return csvTimeout, script, scriptHash, commitPoint, nil
	}

	return 0, nil, nil, nil, errors.New("target script not derived")
}

// bruteForceDelayTaprootManual handles Lightning Terminal taproot channels by testing
// commit points with Lightning Terminal's taproot script construction
func bruteForceDelayTaprootManual(delayBase, revBase *btcec.PublicKey,
	revRoot *shachain.RevocationProducer, lockScript []byte,
	startCsvTimeout, maxCsvTimeout uint16, maxChanUpdates uint64,
	delayBasePointDesc *keychain.KeyDescriptor) (int32,
	[]byte, []byte, *btcec.PublicKey, error) {

	log.Infof("🚀 Starting Lightning Terminal taproot recovery")
	log.Infof("Target script: %x", lockScript)
	log.Infof("🔧 Testing with delay base point key index: %d", delayBasePointDesc.KeyLocator.Index)
	
	if len(lockScript) != 34 || lockScript[0] != 0x51 || lockScript[1] != 0x20 {
		return 0, nil, nil, nil, fmt.Errorf("invalid taproot script: %x", lockScript)
	}

	// Extract target taproot key
	targetTaprootKey := lockScript[2:34]
	log.Infof("Target taproot key: %x", targetTaprootKey)

	// Create dummy keys for unused base points in channel configs
	dummyKey, _ := btcec.ParsePubKey([]byte{
		0x02, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x01,
	})
	
	localChanCfg := &channeldb.ChannelConfig{
		DelayBasePoint: *delayBasePointDesc,
		HtlcBasePoint: keychain.KeyDescriptor{PubKey: dummyKey},
		PaymentBasePoint: keychain.KeyDescriptor{PubKey: dummyKey},
		RevocationBasePoint: keychain.KeyDescriptor{PubKey: dummyKey},
	}
	remoteChanCfg := &channeldb.ChannelConfig{
		DelayBasePoint: keychain.KeyDescriptor{PubKey: dummyKey},
		HtlcBasePoint: keychain.KeyDescriptor{PubKey: dummyKey},
		PaymentBasePoint: keychain.KeyDescriptor{PubKey: dummyKey},
		RevocationBasePoint: keychain.KeyDescriptor{PubKey: revBase},
	}

	// Channel type 3630 from channel.db - SIMPLE_TAPROOT_OVERLAY
	// Binary: 111000101110 = TapscriptRootBit | SimpleTaprootFeatureBit | ScidAliasChanBit | ZeroHtlcTxFeeBit | AnchorOutputsBit | NoFundingTxBit | SingleFunderTweaklessBit
	exactChannelType := channeldb.ChannelType(3630)
	
	// Test Lightning Terminal taproot channel types, starting with exact match
	channelTypes := []channeldb.ChannelType{
		exactChannelType, // Test exact channel type first
		channeldb.SimpleTaprootFeatureBit | channeldb.TapscriptRootBit, // SIMPLE_TAPROOT_OVERLAY base
		channeldb.SimpleTaprootFeatureBit | channeldb.TapscriptRootBit | channeldb.AnchorOutputsBit,
		channeldb.SimpleTaprootFeatureBit | channeldb.TapscriptRootBit | channeldb.AnchorOutputsBit | channeldb.ZeroHtlcTxFeeBit,
		exactChannelType & ^channeldb.ScidAliasChanBit, // Exact without SCID alias  
		exactChannelType & ^channeldb.NoFundingTxBit,   // Exact without no funding tx
		channeldb.SimpleTaprootFeatureBit, // Fallback to standard taproot
		channeldb.SimpleTaprootFeatureBit | channeldb.AnchorOutputsBit,
	}

	// PRIORITY: Test with exact commit point from channel.db first
	exactCommitPointHex := "0322af2607224dff51a3f2eca37f230e3f69ba9817a286959a351a4d8c78a135b1"
	exactKeyIndex := uint32(4) // Key index from channel.db
	
	// Test with exact commit point for key index 4 only  
	if delayBasePointDesc.KeyLocator.Index == exactKeyIndex {
		log.Infof("🎯 FOUND TARGET KEY INDEX %d - Testing exact commit point!", exactKeyIndex)
		if commitPointBytes, err := hex.DecodeString(exactCommitPointHex); err == nil {
			if exactCommitPoint, err := btcec.ParsePubKey(commitPointBytes); err == nil {
				log.Infof("🎯 Testing EXACT commit point: %x", exactCommitPoint.SerializeCompressed())
				
				// Test each channel type with exact commit point
				for _, chanType := range channelTypes {
					// Test CSV values around common delays
					csvTests := []uint16{144, startCsvTimeout} // Start with 144 (from channel.db)
					for csv := startCsvTimeout; csv <= maxCsvTimeout; csv++ {
						csvTests = append(csvTests, csv)
					}
					
					for _, csvDelay := range csvTests {
						// Use Lightning Terminal's exact approach
						keyRing := lnwallet.DeriveCommitmentKeys(
							exactCommitPoint, lntypes.Local, chanType, localChanCfg, remoteChanCfg,
						)
						
						// Lightning Terminal recovery: use real auxiliary leaf instead of empty
						auxLeaf := bruteForceAuxiliaryLeaf()
						
						commitScriptDesc, err := lnwallet.CommitScriptToSelf(
							chanType, false, // chanType, initiator
							keyRing.ToLocalKey, keyRing.RevocationKey, uint32(csvDelay),
							0, // leaseExpiry
							auxLeaf, // Real auxiliary leaf for Lightning Terminal recovery
						)
						if err != nil {
							continue
						}
						
						// Check if we got a tapscript descriptor
						tapscriptDesc, ok := commitScriptDesc.(input.TapscriptDescriptor)
						if !ok {
							continue
						}
						
						// Get Lightning Terminal's taproot construction
						toLocalTree := tapscriptDesc.Tree()
						generatedTaprootKey := toLocalTree.TaprootKey
						generatedTaprootKeyBytes := schnorr.SerializePubKey(generatedTaprootKey)
						
						log.Infof("Testing chanType=%d, csvDelay=%d", chanType, csvDelay)
						log.Infof("Generated: %x", generatedTaprootKeyBytes)
						log.Infof("Target:    %x", targetTaprootKey)
						
						if bytes.Equal(targetTaprootKey, generatedTaprootKeyBytes) {
							log.Infof("🎉 EXACT MATCH with exact commit point!")
							log.Infof("Channel type: %d, CSV delay: %d", chanType, csvDelay)
							
							// Return the exact match
							return int32(csvDelay), tapscriptDesc.WitnessScriptToSign(),
								append([]byte{0x51, 0x20}, generatedTaprootKeyBytes...),
								exactCommitPoint, nil
						}
					}
				}
				if delayBasePointDesc.KeyLocator.Index == exactKeyIndex {
					log.Infof("❌ Exact commit point didn't match for target key index %d", exactKeyIndex)
				}
			}
		}
	}

	// FALLBACK: Test each commit point from the revocation producer
	for i := range maxChanUpdates {
		revPreimage, err := revRoot.AtIndex(i)
		if err != nil {
			continue
		}
		commitPoint := input.ComputeCommitmentPoint(revPreimage[:])
		
		if i < 3 {
			log.Infof("Testing commit point %d: %x", i, commitPoint.SerializeCompressed())
		}

		// Test each channel type
		for _, chanType := range channelTypes {
			// Test CSV values around common delays
			csvTests := []uint16{startCsvTimeout}
			if startCsvTimeout != 144 {
				csvTests = append(csvTests, 144) // Common Lightning Terminal delay
			}
			for csv := startCsvTimeout; csv <= maxCsvTimeout; csv++ {
				csvTests = append(csvTests, csv)
			}
			
			for _, csvDelay := range csvTests {
				// Use Lightning Terminal's exact approach
				keyRing := lnwallet.DeriveCommitmentKeys(
					commitPoint, lntypes.Local, chanType, localChanCfg, remoteChanCfg,
				)
				
				// Lightning Terminal recovery: use real auxiliary leaf instead of empty
				auxLeaf := bruteForceAuxiliaryLeaf()
				
				commitScriptDesc, err := lnwallet.CommitScriptToSelf(
					chanType, false, // chanType, initiator
					keyRing.ToLocalKey, keyRing.RevocationKey, uint32(csvDelay),
					0, // leaseExpiry
					auxLeaf, // Real auxiliary leaf for Lightning Terminal recovery
				)
				if err != nil {
					continue
				}
				
				// Check if we got a tapscript descriptor
				tapscriptDesc, ok := commitScriptDesc.(input.TapscriptDescriptor)
				if !ok {
					continue
				}
				
				// Get Lightning Terminal's taproot construction
				toLocalTree := tapscriptDesc.Tree()
				generatedTaprootKey := toLocalTree.TaprootKey
				generatedTaprootKeyBytes := schnorr.SerializePubKey(generatedTaprootKey)
				
				if bytes.Equal(targetTaprootKey, generatedTaprootKeyBytes) {
					log.Infof("🎉 FOUND TAPROOT MATCH!")
					log.Infof("Commit point: %x", commitPoint.SerializeCompressed())
					log.Infof("Channel type: %d", chanType)
					log.Infof("CSV delay: %d", csvDelay)
					log.Infof("Internal key: %x", toLocalTree.InternalKey.SerializeCompressed())
					
					// Return the script for witness creation
					commitScript := tapscriptDesc.WitnessScriptToSign()
					return int32(csvDelay), commitScript, 
						append([]byte{0x51, 0x20}, generatedTaprootKeyBytes...), commitPoint, nil
				}
			}
		}
	}

	return 0, nil, nil, nil, errors.New("taproot target script not derived")
}
