package main

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightninglabs/chantools/btc"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightninglabs/pool/account"
	"github.com/lightninglabs/pool/poolscript"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lnwallet/chainfee"
	"github.com/spf13/cobra"
)

const (
	poolMainnetFirstBatchBlock = 648168
	defaultMaxNumBlocks        = 200000
	defaultMaxNumAccounts      = 20
	defaultMaxNumBatchKeys     = 500
	oddByte                    = input.PubKeyFormatCompressedOdd
)

var (
	initialBatchKeyBytes, _ = hex.DecodeString(account.InitialBatchKey)
	initialBatchKey, _      = btcec.ParsePubKey(initialBatchKeyBytes)

	mainnetAuctioneerKeyHex = "028e87bdd134238f8347f845d9ecc827b843d0d1e2" +
		"7cdcb46da704d916613f4fce"
)

type closePoolAccountCommand struct {
	APIURL        string
	Outpoint      string
	AuctioneerKey string
	Publish       bool
	SweepAddr     string
	FeeRate       uint32

	MinExpiry       uint32
	MaxNumBlocks    uint32
	MaxNumAccounts  uint32
	MaxNumBatchKeys uint32

	rootKey *rootKey
	cmd     *cobra.Command
}

func newClosePoolAccountCommand() *cobra.Command {
	cc := &closePoolAccountCommand{}
	cc.cmd = &cobra.Command{
		Use:   "closepoolaccount",
		Short: "Tries to close a Pool account that has expired",
		Long: `In case a Pool account cannot be closed normally with the
poold daemon it can be closed with this command. The account **MUST** have
expired already, otherwise this command doesn't work since a signature from the
auctioneer is necessary.

You need to know the account's last unspent outpoint. That can either be
obtained by running 'pool accounts list' `,
		Example: `chantools closepoolaccount \
	--outpoint xxxxxxxxx:y \
	--sweepaddr bc1q..... \
	--feerate 10 \
  	--publish`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.APIURL, "apiurl", defaultAPIURL, "API URL to use (must "+
			"be esplora compatible)",
	)
	cc.cmd.Flags().StringVar(
		&cc.Outpoint, "outpoint", "", "last account outpoint of the "+
			"account to close (<txid>:<txindex>)",
	)
	cc.cmd.Flags().StringVar(
		&cc.AuctioneerKey, "auctioneerkey", mainnetAuctioneerKeyHex,
		"the auctioneer's static public key",
	)
	cc.cmd.Flags().BoolVar(
		&cc.Publish, "publish", false, "publish sweep TX to the chain "+
			"API instead of just printing the TX",
	)
	cc.cmd.Flags().StringVar(
		&cc.SweepAddr, "sweepaddr", "", "address to sweep the funds to",
	)
	cc.cmd.Flags().Uint32Var(
		&cc.FeeRate, "feerate", defaultFeeSatPerVByte, "fee rate to "+
			"use for the sweep transaction in sat/vByte",
	)
	cc.cmd.Flags().Uint32Var(
		&cc.MinExpiry, "minexpiry", poolMainnetFirstBatchBlock,
		"the block to start brute forcing the expiry from",
	)
	cc.cmd.Flags().Uint32Var(
		&cc.MaxNumBlocks, "maxnumblocks", defaultMaxNumBlocks, "the "+
			"maximum number of blocks to try when brute forcing "+
			"the expiry",
	)
	cc.cmd.Flags().Uint32Var(
		&cc.MaxNumAccounts, "maxnumaccounts", defaultMaxNumAccounts,
		"the number of account indices to try at most",
	)
	cc.cmd.Flags().Uint32Var(
		&cc.MaxNumBatchKeys, "maxnumbatchkeys", defaultMaxNumBatchKeys,
		"the number of batch keys to try at most",
	)

	cc.rootKey = newRootKey(cc.cmd, "deriving keys")

	return cc.cmd
}

func (c *closePoolAccountCommand) Execute(_ *cobra.Command, _ []string) error {
	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %w", err)
	}

	// Make sure sweep addr is set.
	if c.SweepAddr == "" {
		return fmt.Errorf("sweep addr is required")
	}

	// Parse account outpoint and auctioneer key.
	outpoint, err := lnd.ParseOutpoint(c.Outpoint)
	if err != nil {
		return fmt.Errorf("error parsing account outpoint: %w", err)
	}

	auctioneerKeyBytes, err := hex.DecodeString(c.AuctioneerKey)
	if err != nil {
		return fmt.Errorf("error decoding auctioneer key: %w", err)
	}

	auctioneerKey, err := btcec.ParsePubKey(auctioneerKeyBytes)
	if err != nil {
		return fmt.Errorf("error parsing auctioneer key: %w", err)
	}

	// Set default values.
	if c.FeeRate == 0 {
		c.FeeRate = defaultFeeSatPerVByte
	}
	return closePoolAccount(
		extendedKey, c.APIURL, outpoint, auctioneerKey, c.SweepAddr,
		c.Publish, c.FeeRate, c.MinExpiry, c.MinExpiry+c.MaxNumBlocks,
		c.MaxNumAccounts, c.MaxNumBatchKeys,
	)
}

func closePoolAccount(extendedKey *hdkeychain.ExtendedKey, apiURL string,
	outpoint *wire.OutPoint, auctioneerKey *btcec.PublicKey,
	sweepAddr string, publish bool, feeRate uint32, minExpiry,
	maxNumBlocks, maxNumAccounts, maxNumBatchKeys uint32) error {

	signer := &lnd.Signer{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}
	api := &btc.ExplorerAPI{BaseURL: apiURL}

	tx, err := api.Transaction(outpoint.Hash.String())
	if err != nil {
		return fmt.Errorf("error looking up TX %s: %w",
			outpoint.Hash.String(), err)
	}

	txOut := tx.Vout[outpoint.Index]
	if txOut.Outspend.Spent {
		return fmt.Errorf("outpoint %v is already spent", outpoint)
	}

	pkScript, err := hex.DecodeString(txOut.ScriptPubkey)
	if err != nil {
		return fmt.Errorf("error decoding pk script %s: %w",
			txOut.ScriptPubkey, err)
	}
	log.Debugf("Brute forcing pk script %x for outpoint %v", pkScript,
		outpoint)

	script, err := txscript.ParsePkScript(pkScript)
	if err != nil {
		return fmt.Errorf("error parsing pk script: %w", err)
	}

	// Let's derive the account key family's extended key first.
	path := []uint32{
		lnd.HardenedKeyStart + uint32(keychain.BIP0043Purpose),
		lnd.HardenedKeyStart + chainParams.HDCoinType,
		lnd.HardenedKeyStart + uint32(poolscript.AccountKeyFamily),
		0,
	}
	accountBaseKey, err := lnd.DeriveChildren(extendedKey, path)
	if err != nil {
		return fmt.Errorf("error deriving account base key: %w", err)
	}

	// Try our luck.
	var (
		acct           *poolAccount
		accountVersion account.Version
	)
	switch script.Class() {
	case txscript.WitnessV0ScriptHashTy:
		accountVersion = account.VersionInitialNoVersion

	case txscript.WitnessV1TaprootTy:
		accountVersion = account.VersionTaprootEnabled

	default:
		return fmt.Errorf("unsupported script class %v", script.Class())
	}

	acct, err = bruteForceAccountScript(
		accountBaseKey, auctioneerKey, minExpiry, maxNumBlocks,
		maxNumAccounts, maxNumBatchKeys, pkScript,
	)
	if err != nil {
		return fmt.Errorf("error brute forcing account script: %w", err)
	}

	log.Debugf("Found pool account %s", acct.String())

	sweepTx := wire.NewMsgTx(2)
	sweepTx.LockTime = acct.expiry
	sweepValue := int64(txOut.Value)

	// Create the transaction input.
	sweepTx.TxIn = []*wire.TxIn{{
		PreviousOutPoint: *outpoint,
	}}

	// Calculate the fee based on the given fee rate and our weight
	// estimation.
	var (
		estimator      input.TxWeightEstimator
		prevOutFetcher = txscript.NewCannedPrevOutputFetcher(
			pkScript, sweepValue,
		)
		signDesc = &input.SignDescriptor{
			KeyDesc: keychain.KeyDescriptor{
				KeyLocator: keychain.KeyLocator{
					Family: poolscript.AccountKeyFamily,
					Index:  acct.keyIndex,
				},
			},
			SingleTweak:   acct.keyTweak,
			WitnessScript: acct.witnessScript,
			Output: &wire.TxOut{
				PkScript: pkScript,
				Value:    sweepValue,
			},
			InputIndex:        0,
			PrevOutputFetcher: prevOutFetcher,
		}
	)

	switch accountVersion {
	case account.VersionInitialNoVersion:
		estimator.AddWitnessInput(poolscript.ExpiryWitnessSize)
		signDesc.HashType = txscript.SigHashAll
		signDesc.SignMethod = input.WitnessV0SignMethod
		signDesc.SigHashes = txscript.NewTxSigHashes(
			sweepTx, prevOutFetcher,
		)

	case account.VersionTaprootEnabled:
		estimator.AddWitnessInput(poolscript.TaprootExpiryWitnessSize)
		signDesc.HashType = txscript.SigHashDefault
		signDesc.SignMethod = input.TaprootScriptSpendSignMethod
	}
	estimator.AddP2WKHOutput()
	feeRateKWeight := chainfee.SatPerKVByte(1000 * feeRate).FeePerKWeight()
	totalFee := feeRateKWeight.FeeForWeight(int64(estimator.Weight()))

	// Add our sweep destination output.
	sweepScript, err := lnd.GetP2WPKHScript(sweepAddr, chainParams)
	if err != nil {
		return err
	}
	sweepTx.TxOut = []*wire.TxOut{{
		Value:    sweepValue - int64(totalFee),
		PkScript: sweepScript,
	}}

	log.Infof("Fee %d sats of %d total amount (estimated weight %d)",
		totalFee, sweepValue, estimator.Weight())

	// Create the sign descriptor for the input then sign the transaction.
	sig, err := signer.SignOutputRaw(sweepTx, signDesc)
	if err != nil {
		return fmt.Errorf("error signing sweep tx: %w", err)
	}

	switch accountVersion {
	case account.VersionInitialNoVersion:
		ourSig := append(sig.Serialize(), byte(signDesc.HashType))
		sweepTx.TxIn[0].Witness = poolscript.SpendExpiry(
			acct.witnessScript, ourSig,
		)

	case account.VersionTaprootEnabled:
		sweepTx.TxIn[0].Witness = poolscript.SpendExpiryTaproot(
			acct.witnessScript, sig.Serialize(), acct.controlBlock,
		)
	}

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

type poolAccount struct {
	keyIndex      uint32
	expiry        uint32
	sharedKey     [32]byte
	batchKey      []byte
	keyTweak      []byte
	witnessScript []byte
	controlBlock  []byte
	version       poolscript.Version
}

func (a *poolAccount) String() string {
	return fmt.Sprintf("key_index=%d, expiry=%d, shared_key=%x, "+
		"batch_key=%x, key_tweak=%x, witness_script=%x, version=%d",
		a.keyIndex, a.expiry, a.sharedKey[:], a.batchKey, a.keyTweak,
		a.witnessScript, a.version)
}

func bruteForceAccountScript(accountBaseKey *hdkeychain.ExtendedKey,
	auctioneerKey *btcec.PublicKey, minExpiry, maxExpiry, maxNumAccounts,
	maxNumBatchKeys uint32, targetScript []byte) (*poolAccount, error) {

	// The outermost loop is over the possible accounts.
	for i := uint32(0); i < maxNumAccounts; i++ {
		accountExtendedKey, err := accountBaseKey.DeriveNonStandard(i)
		if err != nil {
			return nil, fmt.Errorf("error deriving account key: "+
				"%w", err)
		}

		accountPrivKey, err := accountExtendedKey.ECPrivKey()
		if err != nil {
			return nil, fmt.Errorf("error deriving private key: "+
				"%w", err)
		}
		log.Debugf("Trying trader key %x...",
			accountPrivKey.PubKey().SerializeCompressed())

		sharedKey, err := lnd.ECDH(accountPrivKey, auctioneerKey)
		if err != nil {
			return nil, fmt.Errorf("error deriving shared key: "+
				"%w", err)
		}

		// The next loop is over the batch keys.
		batchKeyIndex := uint32(0)
		currentBatchKey := initialBatchKey
		for batchKeyIndex < maxNumBatchKeys {
			// And then finally the loop over the actual account
			// expiry in blocks.
			acct, err := fastScript(
				i, minExpiry, maxExpiry,
				accountPrivKey.PubKey(), auctioneerKey,
				currentBatchKey, sharedKey, targetScript,
			)
			if err == nil {
				return acct, nil
			}
			acct, err = fastScriptTaproot(
				poolscript.VersionTaprootMuSig2, i, minExpiry,
				maxExpiry, accountPrivKey.PubKey(),
				auctioneerKey, currentBatchKey, sharedKey,
				targetScript,
			)
			if err == nil {
				return acct, nil
			}
			acct, err = fastScriptTaproot(
				poolscript.VersionTaprootMuSig2V100RC2, i,
				minExpiry, maxExpiry,
				accountPrivKey.PubKey(), auctioneerKey,
				currentBatchKey, sharedKey, targetScript,
			)
			if err == nil {
				return acct, nil
			}

			currentBatchKey = poolscript.IncrementKey(
				currentBatchKey,
			)
			batchKeyIndex++
		}

		log.Debugf("Tried account index %d of %d", i, maxNumAccounts)
	}

	return nil, fmt.Errorf("account script not derived")
}

func fastScript(keyIndex, expiryFrom, expiryTo uint32, traderKey, auctioneerKey,
	batchKey *btcec.PublicKey, secret [32]byte,
	targetScript []byte) (*poolAccount, error) {

	script, err := txscript.ParsePkScript(targetScript)
	if err != nil {
		return nil, err
	}
	if script.Class() != txscript.WitnessV0ScriptHashTy {
		return nil, fmt.Errorf("incompatible script class")
	}

	traderKeyTweak := poolscript.TraderKeyTweak(batchKey, secret, traderKey)
	tweakedTraderKey := input.TweakPubKeyWithTweak(
		traderKey, traderKeyTweak,
	)
	tweakedAuctioneerKey := input.TweakPubKey(
		auctioneerKey, tweakedTraderKey,
	)

	for block := expiryFrom; block <= expiryTo; block++ {
		builder := txscript.NewScriptBuilder()

		builder.AddData(tweakedTraderKey.SerializeCompressed())
		builder.AddOp(txscript.OP_CHECKSIGVERIFY)

		builder.AddData(tweakedAuctioneerKey.SerializeCompressed())
		builder.AddOp(txscript.OP_CHECKSIG)

		builder.AddOp(txscript.OP_IFDUP)
		builder.AddOp(txscript.OP_NOTIF)
		builder.AddInt64(int64(block))
		builder.AddOp(txscript.OP_CHECKLOCKTIMEVERIFY)
		builder.AddOp(txscript.OP_ENDIF)

		currentScript, err := builder.Script()
		if err != nil {
			return nil, fmt.Errorf("error building script: %w", err)
		}

		currentPkScript, err := input.WitnessScriptHash(currentScript)
		if err != nil {
			return nil, fmt.Errorf("error hashing script: %w", err)
		}
		if !bytes.Equal(currentPkScript, targetScript) {
			continue
		}

		return &poolAccount{
			keyIndex:      keyIndex,
			expiry:        block,
			sharedKey:     secret,
			batchKey:      batchKey.SerializeCompressed(),
			keyTweak:      traderKeyTweak,
			witnessScript: currentScript,
			version:       poolscript.VersionWitnessScript,
		}, nil
	}

	return nil, fmt.Errorf("account script not derived")
}

func fastScriptTaproot(scriptVersion poolscript.Version, keyIndex, expiryFrom,
	expiryTo uint32, traderKey, auctioneerKey, batchKey *btcec.PublicKey,
	secret [32]byte, targetScript []byte) (*poolAccount, error) {

	parsedScript, err := txscript.ParsePkScript(targetScript)
	if err != nil {
		return nil, err
	}
	if parsedScript.Class() != txscript.WitnessV1TaprootTy {
		return nil, fmt.Errorf("incompatible script class")
	}

	traderKeyTweak := poolscript.TraderKeyTweak(batchKey, secret, traderKey)
	tweakedTraderKey := input.TweakPubKeyWithTweak(
		traderKey, traderKeyTweak,
	)

	var muSig2Version input.MuSig2Version
	switch scriptVersion {
	// The v0.4.0 MuSig2 implementation requires the keys to be serialized
	// using the Schnorr (32-byte x-only) serialization format.
	case poolscript.VersionTaprootMuSig2:
		muSig2Version = input.MuSig2Version040

		var err error
		auctioneerKey, err = schnorr.ParsePubKey(
			schnorr.SerializePubKey(auctioneerKey),
		)
		if err != nil {
			return nil, fmt.Errorf("error parsing auctioneer key: "+
				"%w", err)
		}

		traderKey, err = schnorr.ParsePubKey(
			schnorr.SerializePubKey(traderKey),
		)
		if err != nil {
			return nil, fmt.Errorf("error parsing trader key: %w",
				err)
		}

	// The v1.0.0-rc2 MuSig2 implementation works with the regular, 33-byte
	// compressed keys, so we can just pass them in as they are.
	case poolscript.VersionTaprootMuSig2V100RC2:
		muSig2Version = input.MuSig2Version100RC2

	default:
		return nil, fmt.Errorf("invalid account version <%d>",
			scriptVersion)
	}

	for block := expiryFrom; block <= expiryTo; block++ {
		builder := txscript.NewScriptBuilder()

		builder.AddData(schnorr.SerializePubKey(tweakedTraderKey))
		builder.AddOp(txscript.OP_CHECKSIGVERIFY)

		builder.AddInt64(int64(block))
		builder.AddOp(txscript.OP_CHECKLOCKTIMEVERIFY)

		script, err := builder.Script()
		if err != nil {
			return nil, err
		}

		rootHash := txscript.NewBaseTapLeaf(script).TapHash()
		aggregateKey, err := input.MuSig2CombineKeys(
			muSig2Version, []*btcec.PublicKey{
				auctioneerKey, traderKey,
			}, true, &input.MuSig2Tweaks{
				TaprootTweak: rootHash[:],
			},
		)
		if err != nil {
			return nil, fmt.Errorf("error combining keys: %w", err)
		}

		currentKey := schnorr.SerializePubKey(aggregateKey.FinalKey)
		if !bytes.Equal(currentKey, targetScript[2:]) {
			continue
		}

		odd := aggregateKey.FinalKey.SerializeCompressed()[0] == oddByte
		controlBlock := txscript.ControlBlock{
			InternalKey:     aggregateKey.PreTweakedKey,
			LeafVersion:     txscript.BaseLeafVersion,
			OutputKeyYIsOdd: odd,
		}
		blockBytes, err := controlBlock.ToBytes()
		if err != nil {
			return nil, fmt.Errorf("error serializing control "+
				"block: %w", err)
		}

		return &poolAccount{
			keyIndex:      keyIndex,
			expiry:        block,
			sharedKey:     secret,
			batchKey:      batchKey.SerializeCompressed(),
			keyTweak:      traderKeyTweak,
			witnessScript: script,
			controlBlock:  blockBytes,
			version:       scriptVersion,
		}, nil
	}

	return nil, fmt.Errorf("account script not derived")
}
