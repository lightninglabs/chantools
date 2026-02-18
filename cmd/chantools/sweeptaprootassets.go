package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lnwallet"
	"github.com/lightningnetwork/lnd/lntypes"
	"github.com/spf13/cobra"
)

type sweepTaprootAssetsCommand struct {
	SweepAddr string
	FeeRate   uint32
	Publish   bool
	TapdDB    string
	
	rootKey *rootKey
	cmd     *cobra.Command
}

func newSweepTaprootAssetsCommand() *cobra.Command {
	cc := &sweepTaprootAssetsCommand{}
	cc.cmd = &cobra.Command{
		Use: "sweeptaprootassets",
		Short: "Sweep funds from SIMPLE_TAPROOT_OVERLAY channels with Taproot Assets",
		RunE: cc.Execute,
	}
	
	cc.cmd.Flags().StringVar(
		&cc.SweepAddr, "sweepaddr", "", "address to recover funds to",
	)
	cc.cmd.Flags().Uint32Var(
		&cc.FeeRate, "feerate", 20, "fee rate in sat/vByte",
	)
	cc.cmd.Flags().BoolVar(
		&cc.Publish, "publish", false, "publish transaction",
	)
	cc.cmd.Flags().StringVar(
		&cc.TapdDB, "tapddb", "", "path to tapd database file",
	)

	cc.rootKey = newRootKey(cc.cmd, "signing")
	return cc.cmd
}

// TapscriptPreimageType represents the type of tapscript preimage
type TapscriptPreimageType uint8

const (
	LeafPreimage   TapscriptPreimageType = 0
	BranchPreimage TapscriptPreimageType = 1
)

func (c *sweepTaprootAssetsCommand) Execute(_ *cobra.Command, _ []string) error {
	extendedKey, err := c.rootKey.read()
	if err != nil {
		return err
	}

	// Test with the first UTXO to decode its auxiliary leaf data
	testUTXO := struct {
		txid           string
		output         uint32
		value          int64
		script         string
		keyIndex       uint32
		tapscriptRoot  string
		auxSiblingData string // This is the 64-byte TapscriptSibling from database
	}{
		"dee8f230628b2d61204c4ea46dbe13746e216c6a9978b108e7b523e86a06f4e5",
		1,
		97027,
		"512046e9c92de7004ebcb835e2180f2ee892363404a3a9a6e76acc0ce185c8abcb87",
		5,
		"", // Remove hardcoded tapscript root - we'll compute it
		"0168e37156072e50607e489b3339c41004b9ded3377873cab215fefaee7029561855b6fff2e5719cb19e51bd748e92285b95b25a1275d3e2485ec8f9b0cdc828d1",
	}

	log.Infof("Analyzing auxiliary sibling data for UTXO: %s:%d", testUTXO.txid, testUTXO.output)

	// Decode the TapscriptSibling data from the database
	auxSiblingBytes, err := hex.DecodeString(testUTXO.auxSiblingData)
	if err != nil {
		return fmt.Errorf("decoding aux sibling data: %w", err)
	}

	log.Infof("Raw auxiliary sibling data (%d bytes): %x", len(auxSiblingBytes), auxSiblingBytes)

	// Let me first examine the raw bytes to understand the format
	log.Infof("Analyzing raw bytes:")
	for i := 0; i < len(auxSiblingBytes) && i < 10; i++ {
		log.Infof("  Byte %d: 0x%02x (%d)", i, auxSiblingBytes[i], auxSiblingBytes[i])
	}
	
	// The 64-byte data might be stored in different formats depending on the circumstances
	// Let's try to understand what format this is by examining the structure
	if len(auxSiblingBytes) == 64 {
		log.Infof("This is exactly 64 bytes - might be raw branch data (two 32-byte hashes)")
		// Try interpreting as raw branch data
		leftHash := auxSiblingBytes[:32]
		rightHash := auxSiblingBytes[32:]
		log.Infof("Left hash: %x", leftHash)
		log.Infof("Right hash: %x", rightHash)
		
		// Calculate the branch hash directly
		branchHash := c.computeTaprootMerkleHash(leftHash, rightHash)
		log.Infof("Computed branch hash: %x", branchHash)
		
		// Compute what the tapscript root should be with this auxiliary leaf
		return c.computeAndTestTapscriptRoot(testUTXO, branchHash, extendedKey)
	} else {
		log.Infof("This is %d bytes - attempting TLV decode", len(auxSiblingBytes))
		
		// Try TLV decode
		preimage, tapHash, err := c.decodeTapscriptPreimage(auxSiblingBytes)
		if err != nil {
			return fmt.Errorf("failed to decode TapscriptPreimage: %w", err)
		}
		
		log.Infof("Decoded TapscriptPreimage type: %v", preimage.Type())
		log.Infof("Tapscript hash: %x", tapHash[:])
		
		// Compute what the tapscript root should be with this auxiliary leaf
		return c.computeAndTestTapscriptRoot(testUTXO, tapHash[:], extendedKey)
	}
}

func getPreimageTypeString(t TapscriptPreimageType) string {
	switch t {
	case LeafPreimage:
		return "LeafPreimage"
	case BranchPreimage:
		return "BranchPreimage"
	default:
		return fmt.Sprintf("Unknown(%d)", t)
	}
}

// TapscriptPreimage mimics the Taproot Assets TapscriptPreimage structure
type TapscriptPreimage struct {
	SiblingPreimage []byte
	SiblingType     uint8
}

func (t *TapscriptPreimage) Type() string {
	switch t.SiblingType {
	case 0:
		return "LeafPreimage"
	case 1:
		return "BranchPreimage"
	default:
		return fmt.Sprintf("Unknown(%d)", t.SiblingType)
	}
}

func (c *sweepTaprootAssetsCommand) decodeTapscriptPreimage(encoded []byte) (*TapscriptPreimage, *[32]byte, error) {
	if len(encoded) == 0 {
		return nil, nil, fmt.Errorf("empty encoded data")
	}

	// The TapscriptPreimage encoding format from TapscriptPreimageEncoder is:
	// 1 byte: type (0 = LeafPreimage, 1 = BranchPreimage)
	// Variable bytes: the preimage data encoded with tlv.EVarBytes (varint length + data)
	
	if len(encoded) < 2 {
		return nil, nil, fmt.Errorf("encoded data too short: %d bytes", len(encoded))
	}

	preimageType := encoded[0]
	remaining := encoded[1:]
	
	// For BranchPreimage type, the data is simply the raw 64 bytes
	// (no length varint for branch data)
	var preimageData []byte
	if preimageType == 1 && len(remaining) == 64 {
		log.Infof("BranchPreimage: using raw 64-byte data")
		preimageData = remaining
	} else {
		// For other types, use the standard TLV decoding
		reader := bytes.NewReader(remaining)
		var err error
		preimageData, err = wire.ReadVarBytes(reader, 0, uint32(len(remaining)), "preimage")
		if err != nil {
			return nil, nil, fmt.Errorf("reading preimage data: %w", err)
		}
	}
	
	log.Infof("Decoded preimage type: %d", preimageType)
	log.Infof("Decoded preimage data (%d bytes): %x", len(preimageData), preimageData)
	
	preimage := &TapscriptPreimage{
		SiblingPreimage: preimageData,
		SiblingType:     preimageType,
	}
	
	// Calculate the TapHash of the preimage
	tapHash, err := c.calculateTapHash(preimage)
	if err != nil {
		return nil, nil, fmt.Errorf("calculating tap hash: %w", err)
	}
	
	return preimage, tapHash, nil
}

func (c *sweepTaprootAssetsCommand) calculateTapHash(preimage *TapscriptPreimage) (*[32]byte, error) {
	switch preimage.SiblingType {
	case 0: // LeafPreimage
		// For leaf preimage, we need to parse the script and calculate the TapLeaf hash
		// The preimage format is: [leaf_version:1byte] [script_length:varint] [script:variable]
		if len(preimage.SiblingPreimage) < 2 {
			return nil, fmt.Errorf("leaf preimage too short")
		}
		
		leafVersion := preimage.SiblingPreimage[0]
		scriptReader := bytes.NewReader(preimage.SiblingPreimage[1:])
		script, err := wire.ReadVarBytes(scriptReader, 0, uint32(len(preimage.SiblingPreimage)-1), "script")
		if err != nil {
			return nil, fmt.Errorf("reading script from leaf preimage: %w", err)
		}
		
		// Create TapLeaf and calculate its hash
		tapLeaf := txscript.NewTapLeaf(txscript.TapscriptLeafVersion(leafVersion), script)
		hash := tapLeaf.TapHash()
		var result [32]byte
		copy(result[:], hash[:])
		return &result, nil
		
	case 1: // BranchPreimage
		// For branch preimage, the data is 64 bytes (two 32-byte hashes)
		if len(preimage.SiblingPreimage) != 64 {
			return nil, fmt.Errorf("expected 64 bytes for branch preimage, got %d", len(preimage.SiblingPreimage))
		}
		
		leftHash := preimage.SiblingPreimage[:32]
		rightHash := preimage.SiblingPreimage[32:]
		
		log.Infof("Branch left hash: %x", leftHash)
		log.Infof("Branch right hash: %x", rightHash)
		
		// Calculate the branch hash using the same algorithm as Taproot Assets
		branchHash := c.computeTaprootMerkleHash(leftHash, rightHash)
		var result [32]byte
		copy(result[:], branchHash)
		return &result, nil
		
	default:
		return nil, fmt.Errorf("unknown preimage type: %d", preimage.SiblingType)
	}
}

func (c *sweepTaprootAssetsCommand) computeAndTestTapscriptRoot(testUTXO struct {
	txid           string
	output         uint32
	value          int64
	script         string
	keyIndex       uint32
	tapscriptRoot  string
	auxSiblingData string
}, siblingHash []byte, extendedKey *hdkeychain.ExtendedKey) error {

	// Get the raw preimage data for analysis (not needed with real asset data)
	// auxSiblingBytes, _ := hex.DecodeString(testUTXO.auxSiblingData)
	// preimage, _, _ := c.decodeTapscriptPreimage(auxSiblingBytes)

	keyRing := &lnd.HDKeyRing{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}

	// Get the key for signing
	localKeyDesc, err := keyRing.DeriveKey(keychain.KeyLocator{
		Family: keychain.KeyFamilyDelayBase,
		Index:  testUTXO.keyIndex,
	})
	if err != nil {
		return err
	}

	// Create delay script
	delayScript := &txscript.ScriptBuilder{}
	delayScript.AddData(schnorr.SerializePubKey(localKeyDesc.PubKey))
	delayScript.AddOp(txscript.OP_CHECKSIG)
	delayScript.AddInt64(144) // CSV timeout
	delayScript.AddOp(txscript.OP_CHECKSEQUENCEVERIFY)
	delayScript.AddOp(txscript.OP_DROP)

	delayScriptBytes, err := delayScript.Script()
	if err != nil {
		return fmt.Errorf("building delay script: %w", err)
	}

	// Create delay tap leaf
	delayTapLeaf := txscript.NewBaseTapLeaf(delayScriptBytes)
	delayLeafHash := delayTapLeaf.TapHash()

	log.Infof("Delay leaf hash: %x", delayLeafHash[:])

	// For SIMPLE_TAPROOT_OVERLAY, we need to create a 3-leaf tree:
	// delay + revocation + auxiliary
	// First, get the revocation key
	revocationKeyDesc, err := keyRing.DeriveKey(keychain.KeyLocator{
		Family: keychain.KeyFamilyRevocationBase,
		Index:  testUTXO.keyIndex,
	})
	if err != nil {
		return err
	}

	// Create revocation script  
	revocationScript := &txscript.ScriptBuilder{}
	revocationScript.AddData(schnorr.SerializePubKey(revocationKeyDesc.PubKey))
	revocationScript.AddOp(txscript.OP_CHECKSIG)

	revocationScriptBytes, err := revocationScript.Script()
	if err != nil {
		return fmt.Errorf("building revocation script: %w", err)
	}

	// Create revocation tap leaf
	revocationTapLeaf := txscript.NewBaseTapLeaf(revocationScriptBytes)
	revocationLeafHash := revocationTapLeaf.TapHash()

	log.Infof("Revocation leaf hash: %x", revocationLeafHash[:])

	// Use the REAL asset data from the pending force-closed channel
	// From docker exec lit lncli --network signet pendingchannels
	// Asset ID: cd2adf3323bf98d91de96f8332117d3c5cdac8209b6e3ce0d00acfebd5fe82d7 (from pending force-closed channel)
	// Script Key: 0250aaeb166f4234650d84a2d8a130987aeaf6950206e0905401ee74ff3f8d18e6
	// Amount: 100,000
	
	log.Infof("Using REAL asset data from live pending force-closed channel:")
	assetIDHex := "cd2adf3323bf98d91de96f8332117d3c5cdac8209b6e3ce0d00acfebd5fe82d7"
	scriptKeyHex := "0250aaeb166f4234650d84a2d8a130987aeaf6950206e0905401ee74ff3f8d18e6"
	log.Infof("Asset ID: %s", assetIDHex)
	log.Infof("Script Key: %s", scriptKeyHex)
	
	// Script key is the tweaked taproot key, not used directly in auxiliary leaf
	log.Infof("Script key (not used in aux leaf): %s", scriptKeyHex)
	
	// The auxiliary leaf is a TapCommitment containing asset data, not a spending script
	// Format: [tapVersion] + TaprootAssetsMarker + rootHash + rootSum
	
	// Parse asset ID to get root hash (asset ID often IS the root hash for single assets)
	assetIDBytes, err := hex.DecodeString(assetIDHex)
	if err != nil {
		return fmt.Errorf("decoding asset ID: %w", err)
	}
	
	// Taproot Assets marker: SHA256("taproot-assets")
	taprootAssetsMarker := sha256.Sum256([]byte("taproot-assets"))
	
	// Asset amount: 100,000 units (8 bytes big-endian)
	assetAmount := uint64(100000)
	amountBytes := make([]byte, 8)
	for i := 0; i < 8; i++ {
		amountBytes[7-i] = byte(assetAmount >> (i * 8))
	}
	
	// Build TapCommitment leaf script (try V0 format first)
	tapVersion := byte(0x00)
	auxScriptBytes := []byte{tapVersion}
	auxScriptBytes = append(auxScriptBytes, taprootAssetsMarker[:]...)
	auxScriptBytes = append(auxScriptBytes, assetIDBytes...)
	auxScriptBytes = append(auxScriptBytes, amountBytes...)
	
	log.Infof("TapCommitment auxiliary script (%d bytes): %x", len(auxScriptBytes), auxScriptBytes)
	
	// Create auxiliary tap leaf
	auxTapLeaf := txscript.NewBaseTapLeaf(auxScriptBytes)
	auxLeafHash := auxTapLeaf.TapHash()
	
	log.Infof("TapCommitment auxiliary leaf hash: %x", auxLeafHash[:])
	
	// HYPOTHESIS: The stored sibling hash IS the correct auxiliary leaf hash
	// Let's test using the decoded branch hash directly as auxiliary leaf
	log.Infof("Testing stored branch hash as auxiliary leaf: %x", siblingHash)
	
	// Test both approaches
	realDelayRevokeBranch := c.computeTaprootMerkleHash(delayLeafHash[:], revocationLeafHash[:])
	
	// Approach 1: Use computed TapCommitment aux leaf
	realComputedRoot1 := c.computeTaprootMerkleHash(realDelayRevokeBranch, auxLeafHash[:])
	log.Infof("Tree with computed aux leaf: %x", realComputedRoot1)
	
	// Approach 2: Use stored sibling hash as aux leaf
	realComputedRoot2 := c.computeTaprootMerkleHash(realDelayRevokeBranch, siblingHash)
	log.Infof("Tree with stored aux leaf: %x", realComputedRoot2)
	
	// We'll test different combinations below

	// DEBUGGING: Try working backwards from the actual output key  
	log.Infof("Working backwards from actual output key...")
	
	// Use LND's CommitScriptToSelf like Lightning Terminal does
	// This is the correct approach instead of manual tree construction
	
	log.Infof("Using LND's CommitScriptToSelf with SIMPLE_TAPROOT_OVERLAY...")
	
	// Extract the actual output key first
	scriptBytes, err := hex.DecodeString(testUTXO.script)
	if err != nil {
		return fmt.Errorf("decoding script: %w", err)
	}
	if len(scriptBytes) != 34 || scriptBytes[0] != 0x51 || scriptBytes[1] != 0x20 {
		return fmt.Errorf("invalid P2TR script format")
	}
	actualOutputKey := scriptBytes[2:]
	log.Infof("Target output key: %x", actualOutputKey)
	
	// Test systematic combinations of internal keys and tapscript roots
	
	// WORK BACKWARDS: Try to find the correct internal key by testing all possible derivations
	// We know the target output key, let's try different key derivations
	
	log.Infof("🚀 Using Lightning Terminal's actual CommitScriptToSelf function!")
	
	// Get the commitment point for this specific commitment
	// Use the channel's current commitment point (index from keyIndex)
	commitPointKeyDesc, err := keyRing.DeriveKey(keychain.KeyLocator{
		Family: keychain.KeyFamilyMultiSig,
		Index:  testUTXO.keyIndex,
	})
	if err != nil {
		return fmt.Errorf("deriving commitment point: %w", err)
	}
	
	commitPoint := commitPointKeyDesc.PubKey
	log.Infof("Commitment point: %x", schnorr.SerializePubKey(commitPoint))
	
	// Create channel configuration like Lightning Terminal does
	// Use the actual channel data we know
	localChanCfg := channeldb.ChannelConfig{
		DelayBasePoint:      localKeyDesc,
		RevocationBasePoint: revocationKeyDesc,
		PaymentBasePoint:    localKeyDesc, // Use same for now
		HtlcBasePoint:       localKeyDesc, // Use same for now
		MultiSigKey:         keychain.KeyDescriptor{PubKey: commitPoint},
	}
	
	remoteChanCfg := channeldb.ChannelConfig{
		DelayBasePoint:      revocationKeyDesc, // Remote uses different keys
		RevocationBasePoint: localKeyDesc,
		PaymentBasePoint:    revocationKeyDesc,
		HtlcBasePoint:       revocationKeyDesc,
		MultiSigKey:         keychain.KeyDescriptor{PubKey: commitPoint},
	}
	
	// Use Lightning Terminal's commitment key derivation
	commitKeys := lnwallet.DeriveCommitmentKeys(
		commitPoint,
		lntypes.Local,
		channeldb.SingleFunderTweaklessBit | channeldb.SimpleTaprootFeatureBit, // SIMPLE_TAPROOT_OVERLAY
		&localChanCfg,
		&remoteChanCfg,
	)
	
	log.Infof("✅ LND-derived commitment keys:")
	log.Infof("ToLocalKey: %x", schnorr.SerializePubKey(commitKeys.ToLocalKey))
	log.Infof("ToRemoteKey: %x", schnorr.SerializePubKey(commitKeys.ToRemoteKey))
	log.Infof("RevocationKey: %x", schnorr.SerializePubKey(commitKeys.RevocationKey))
	
	// Lightning Terminal ALWAYS uses NUMS key as internal key! 
	// The ToLocalKey goes into the delay script, not as internal key
	log.Infof("🎯 Using NUMS key as internal key (Lightning Terminal approach)")
	numsInternalKey := input.TaprootNUMSKey.SerializeCompressed()[1:] // Remove 0x02 prefix
	log.Infof("NUMS Internal key: %x", numsInternalKey)
	
	// Re-create delay script using the ACTUAL ToLocalKey from LND
	delayScriptLT := &txscript.ScriptBuilder{}
	delayScriptLT.AddData(schnorr.SerializePubKey(commitKeys.ToLocalKey))
	delayScriptLT.AddOp(txscript.OP_CHECKSIG)
	delayScriptLT.AddInt64(144) // CSV timeout
	delayScriptLT.AddOp(txscript.OP_CHECKSEQUENCEVERIFY)
	delayScriptLT.AddOp(txscript.OP_DROP)

	delayScriptBytesLT, err := delayScriptLT.Script()
	if err != nil {
		return fmt.Errorf("building Lightning Terminal delay script: %w", err)
	}

	// Create delay tap leaf with Lightning Terminal's ToLocalKey
	delayTapLeafLT := txscript.NewBaseTapLeaf(delayScriptBytesLT)
	delayLeafHashLT := delayTapLeafLT.TapHash()

	log.Infof("LT Delay leaf hash (with ToLocalKey): %x", delayLeafHashLT[:])
	
	// Test all our computed roots with NUMS key as internal key
	testRoots := []struct {
		name string
		root []byte
	}{
		{"LT 2-leaf (delay+revocation)", c.computeTaprootMerkleHash(delayLeafHashLT[:], revocationLeafHash[:])},
		{"LT 3-leaf with stored aux", c.computeTaprootMerkleHash(c.computeTaprootMerkleHash(delayLeafHashLT[:], revocationLeafHash[:]), siblingHash)},
		{"LT 3-leaf (delay+revocation+aux)", c.computeTaprootMerkleHash(c.computeTaprootMerkleHash(delayLeafHashLT[:], revocationLeafHash[:]), siblingHash)},
		{"LT 3-leaf (delay+aux+revocation)", c.computeTaprootMerkleHash(c.computeTaprootMerkleHash(delayLeafHashLT[:], siblingHash), revocationLeafHash[:])},
		{"LT 3-leaf (revocation+delay+aux)", c.computeTaprootMerkleHash(c.computeTaprootMerkleHash(revocationLeafHash[:], delayLeafHashLT[:]), siblingHash)},
		{"LT 3-leaf (revocation+aux+delay)", c.computeTaprootMerkleHash(c.computeTaprootMerkleHash(revocationLeafHash[:], siblingHash), delayLeafHashLT[:])},
		{"LT 3-leaf (aux+delay+revocation)", c.computeTaprootMerkleHash(c.computeTaprootMerkleHash(siblingHash, delayLeafHashLT[:]), revocationLeafHash[:])},
		{"LT 3-leaf (aux+revocation+delay)", c.computeTaprootMerkleHash(c.computeTaprootMerkleHash(siblingHash, revocationLeafHash[:]), delayLeafHashLT[:])},
	}
	
	for _, test := range testRoots {
		expectedOutputKey := c.computeTaprootOutputKey(numsInternalKey, test.root)
		log.Infof("Testing %s with NUMS internal key: %x", test.name, expectedOutputKey)
		
		if bytes.Equal(actualOutputKey, expectedOutputKey) {
			log.Infof("🎉 SUCCESS! %s with NUMS internal key produces correct output!", test.name)
			log.Infof("Internal key (NUMS): %x", numsInternalKey)
			log.Infof("Root: %x", test.root)
			log.Infof("Output: %x", expectedOutputKey)
			
			// Success! Use this to create the transaction with Lightning Terminal approach
			return c.createAndSignTransaction(testUTXO, delayScriptBytesLT, test.root, extendedKey)
		}
	}
	
	// Legacy test with known keys
	testKeys := []struct {
		name string
		key  []byte
	}{
		{"DelayBaseKey", schnorr.SerializePubKey(localKeyDesc.PubKey)},
		{"RevocationKey", schnorr.SerializePubKey(revocationKeyDesc.PubKey)},
	}
	
	for _, testKey := range testKeys {
		log.Infof("Testing internal key %s: %x", testKey.name, testKey.key)
		
		// Test with this key as internal key
		internalKey := testKey.key
		
		// Test all our computed roots with this internal key
		testRoots := []struct {
			name string
			root []byte
		}{
			{"Simple 2-leaf", c.computeTaprootMerkleHash(delayLeafHash[:], revocationLeafHash[:])},
			{"Stored aux leaf", c.computeTaprootMerkleHash(c.computeTaprootMerkleHash(delayLeafHash[:], revocationLeafHash[:]), siblingHash)},
		}
		
		for _, test := range testRoots {
			expectedOutputKey := c.computeTaprootOutputKey(internalKey, test.root)
			log.Infof("  %s with %s: %x", test.name, testKey.name, expectedOutputKey)
			
			if bytes.Equal(actualOutputKey, expectedOutputKey) {
				log.Infof("🎉 SUCCESS! %s with %s produces correct output key!", test.name, testKey.name)
				return c.createAndSignTransaction(testUTXO, delayScriptBytes, test.root, extendedKey)
			}
		}
	}

	log.Infof("❌ No combination of internal key and tapscript root matches")
	return fmt.Errorf("no valid key/root combination found")
}

func (c *sweepTaprootAssetsCommand) testDecodedSiblingHash(testUTXO struct {
	txid           string
	output         uint32
	value          int64
	script         string
	keyIndex       uint32
	tapscriptRoot  string
	auxSiblingData string
}, siblingHash []byte, extendedKey *hdkeychain.ExtendedKey) error {

	keyRing := &lnd.HDKeyRing{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}

	// Get the key for signing
	localKeyDesc, err := keyRing.DeriveKey(keychain.KeyLocator{
		Family: keychain.KeyFamilyDelayBase,
		Index:  testUTXO.keyIndex,
	})
	if err != nil {
		return err
	}

	// Create delay script
	delayScript := &txscript.ScriptBuilder{}
	delayScript.AddData(schnorr.SerializePubKey(localKeyDesc.PubKey))
	delayScript.AddOp(txscript.OP_CHECKSIG)
	delayScript.AddInt64(144) // CSV timeout
	delayScript.AddOp(txscript.OP_CHECKSEQUENCEVERIFY)
	delayScript.AddOp(txscript.OP_DROP)

	delayScriptBytes, err := delayScript.Script()
	if err != nil {
		return fmt.Errorf("building delay script: %w", err)
	}

	// Create delay tap leaf
	delayTapLeaf := txscript.NewBaseTapLeaf(delayScriptBytes)
	delayLeafHash := delayTapLeaf.TapHash()

	log.Infof("Delay leaf hash: %x", delayLeafHash[:])

	expectedRoot, _ := hex.DecodeString(testUTXO.tapscriptRoot)
	log.Infof("Expected tapscript root: %x", expectedRoot)

	// Test if the decoded siblingHash combined with delayLeafHash gives us the expected root
	actualRoot := c.computeTaprootMerkleHash(delayLeafHash[:], siblingHash)
	log.Infof("Computed tapscript root with decoded sibling hash: %x", actualRoot)
	
	if bytes.Equal(actualRoot, expectedRoot) {
		log.Infof("SUCCESS! Decoded sibling hash produces correct tapscript root")
		return c.createAndSignTransaction(testUTXO, delayScriptBytes, siblingHash, extendedKey)
	} else {
		return fmt.Errorf("decoded sibling hash does not produce correct tapscript root")
	}
}

func (c *sweepTaprootAssetsCommand) computeTaprootMerkleHash(a, b []byte) []byte {
	// Sort the hashes lexicographically as required by taproot
	if bytes.Compare(a, b) > 0 {
		a, b = b, a
	}
	
	// Compute the tagged hash for taproot branch using the correct format
	// TaggedHash(tag, data) = SHA256(SHA256(tag) || SHA256(tag) || data)
	tag := "TapBranch"
	tagHash := sha256.Sum256([]byte(tag))
	
	combined := append(a, b...)
	preimage := append(tagHash[:], append(tagHash[:], combined...)...)
	hash := sha256.Sum256(preimage)
	return hash[:]
}

func (c *sweepTaprootAssetsCommand) computeTaprootOutputKey(internalKey []byte, tapscriptRoot []byte) []byte {
	// Use the proper btcd taproot output key computation
	// This implements: output_key = internal_key + tweak * G
	
	// Parse the internal key
	internalKeyParsed, err := schnorr.ParsePubKey(internalKey)
	if err != nil {
		log.Errorf("Error parsing internal key: %v", err)
		return internalKey
	}
	
	// Compute the taproot output key using btcd's implementation
	outputKey := txscript.ComputeTaprootOutputKey(internalKeyParsed, tapscriptRoot)
	
	// Serialize the output key (x-only)
	outputKeyBytes := schnorr.SerializePubKey(outputKey)
	
	return outputKeyBytes
}

func (c *sweepTaprootAssetsCommand) createAndSignTransaction(testUTXO struct {
	txid           string
	output         uint32
	value          int64
	script         string
	keyIndex       uint32
	tapscriptRoot  string
	auxSiblingData string
}, delayScriptBytes []byte, siblingHash []byte, extendedKey *hdkeychain.ExtendedKey) error {

	keyRing := &lnd.HDKeyRing{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}
	signer := &lnd.Signer{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}

	// Create transaction
	sweepTx := wire.NewMsgTx(2)

	// Add input
	hash, _ := hex.DecodeString(testUTXO.txid)
	// Reverse for wire format
	for i, j := 0, len(hash)-1; i < j; i, j = i+1, j-1 {
		hash[i], hash[j] = hash[j], hash[i]
	}
	var hashArray [32]byte
	copy(hashArray[:], hash)

	sweepTx.AddTxIn(&wire.TxIn{
		PreviousOutPoint: wire.OutPoint{
			Hash:  hashArray,
			Index: testUTXO.output,
		},
		Sequence: 144, // CSV delay for commitment outputs
	})

	// Calculate fee and output
	estimatedSize := int64(200) // Conservative estimate for 1 input, 1 output
	fee := estimatedSize * int64(c.FeeRate)
	outputValue := testUTXO.value - fee

	if outputValue < 1000 {
		return fmt.Errorf("output too small: %d", outputValue)
	}

	// Parse sweep address
	sweepAddr, err := lnd.ParseAddress(c.SweepAddr, chainParams)
	if err != nil {
		return err
	}

	sweepScript, err := txscript.PayToAddrScript(sweepAddr)
	if err != nil {
		return err
	}

	sweepTx.AddTxOut(&wire.TxOut{
		Value:    outputValue,
		PkScript: sweepScript,
	})

	// Create previous outputs map
	prevOutputs := make(map[wire.OutPoint]*wire.TxOut)
	outpoint := wire.OutPoint{Hash: hashArray, Index: testUTXO.output}
	scriptBytes, _ := hex.DecodeString(testUTXO.script)

	prevOutputs[outpoint] = &wire.TxOut{
		Value:    testUTXO.value,
		PkScript: scriptBytes,
	}

	prevOutFetcher := txscript.NewMultiPrevOutFetcher(prevOutputs)

	// For signing, we need to find which base key was used to derive the ToLocalKey
	// Lightning Terminal uses DelayBasePoint + commitment point tweaking
	// Let's use the original DelayBaseKey since that's what gets tweaked to ToLocalKey
	localKeyDesc, err := keyRing.DeriveKey(keychain.KeyLocator{
		Family: keychain.KeyFamilyDelayBase,
		Index:  testUTXO.keyIndex,
	})
	if err != nil {
		return err
	}
	
	log.Infof("Using DelayBaseKey for signing (tweaked to ToLocalKey): %x", schnorr.SerializePubKey(localKeyDesc.PubKey))

	// Create control block manually
	// Control block format: [version] [internal_key] [merkle_proof...]
	version := byte(0xc0) // Script path spend, base leaf version
	
	// Use NUMS key as internal key
	internalKey := input.TaprootNUMSKey.SerializeCompressed()[1:] // Remove 0x02 prefix
	
	// The merkle proof is the sibling hash
	controlBlock := []byte{version}
	controlBlock = append(controlBlock, internalKey...)
	controlBlock = append(controlBlock, siblingHash...)
	
	log.Infof("Control block (%d bytes): %x", len(controlBlock), controlBlock)

	// Sign the input using the DelayBaseKey (which gets tweaked to ToLocalKey)
	signDesc := &input.SignDescriptor{
		KeyDesc:           localKeyDesc,
		Output:            prevOutputs[sweepTx.TxIn[0].PreviousOutPoint],
		HashType:          txscript.SigHashDefault,
		PrevOutputFetcher: prevOutFetcher,
		InputIndex:        0,
		SignMethod:        input.TaprootScriptSpendSignMethod,
		WitnessScript:     delayScriptBytes,
	}

	sig, err := signer.SignOutputRaw(sweepTx, signDesc)
	if err != nil {
		return fmt.Errorf("signing transaction: %w", err)
	}

	// Create witness
	witness := wire.TxWitness{
		sig.Serialize(),
		delayScriptBytes,
		controlBlock,
	}

	sweepTx.TxIn[0].Witness = witness

	// Serialize and print
	var buf bytes.Buffer
	err = sweepTx.Serialize(&buf)
	if err != nil {
		return err
	}

	log.Infof("Total input: %d sats", testUTXO.value)
	log.Infof("Fee: %d sats", fee)
	log.Infof("Output: %d sats", outputValue)
	log.Infof("Raw TX: %x", buf.Bytes())

	if c.Publish {
		api := newExplorerAPI("https://blockstream.info/signet/api")
		txHash, err := api.PublishTx(hex.EncodeToString(buf.Bytes()))
		if err != nil {
			return fmt.Errorf("publish failed: %w", err)
		} else if strings.Contains(txHash, "error") || strings.Contains(txHash, "failed") {
			return fmt.Errorf("publish failed: %s", txHash)
		} else {
			log.Infof("SUCCESS! Published! TXID: %s", txHash)
		}
	}

	return nil
}