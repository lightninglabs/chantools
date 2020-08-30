package main

import (
	"bytes"
	"encoding/hex"
	"fmt"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/guggero/chantools/btc"
	"github.com/guggero/chantools/lnd"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lnwallet/chainfee"
	"github.com/lightningnetwork/lnd/shachain"
)

const (
	keyBasePath = "m/1017'/%d'"
	maxKeys     = 500
	maxPoints   = 500
)

type sweepTimeLockManualCommand struct {
	RootKey                   string `long:"rootkey" description:"BIP32 HD root key to use. Leave empty to prompt for lnd 24 word aezeed."`
	Publish                   bool   `long:"publish" description:"Should the sweep TX be published to the chain API?"`
	SweepAddr                 string `long:"sweepaddr" description:"The address the funds should be sweeped to."`
	MaxCsvLimit               int    `long:"maxcsvlimit" description:"Maximum CSV limit to use. (default 2000)"`
	FeeRate                   uint32 `long:"feerate" description:"The fee rate to use for the sweep transaction in sat/vByte. (default 2 sat/vByte)"`
	TimeLockAddr              string `long:"timelockaddr" description:"The address of the time locked commitment output where the funds are stuck in."`
	RemoteRevocationBasePoint string `long:"remoterevbasepoint" description:"The remote's revocation base point, can be found in a channel.backup file."`
}

func (c *sweepTimeLockManualCommand) Execute(_ []string) error {
	setupChainParams(cfg)

	var (
		extendedKey *hdkeychain.ExtendedKey
		err         error
	)

	// Check that root key is valid or fall back to console input.
	switch {
	case c.RootKey != "":
		extendedKey, err = hdkeychain.NewKeyFromString(c.RootKey)

	default:
		extendedKey, _, err = lnd.ReadAezeedFromTerminal(chainParams)
	}
	if err != nil {
		return fmt.Errorf("error reading root key: %v", err)
	}

	// Make sure the sweep and time lock addrs are set.
	if c.SweepAddr == "" {
		return fmt.Errorf("sweep addr is required")
	}
	if c.TimeLockAddr == "" {
		return fmt.Errorf("time lock addr is required")
	}

	// The remote revocation base point must also be set and a valid EC
	// point.
	remoteRevPoint, err := pubKeyFromHex(c.RemoteRevocationBasePoint)
	if err != nil {
		return fmt.Errorf("invalid remote revocation base point: %v",
			err)
	}

	// Set default values.
	if c.MaxCsvLimit == 0 {
		c.MaxCsvLimit = defaultCsvLimit
	}
	if c.FeeRate == 0 {
		c.FeeRate = defaultFeeSatPerVByte
	}
	return sweepTimeLockManual(
		extendedKey, cfg.APIURL, c.SweepAddr, c.TimeLockAddr,
		remoteRevPoint, c.MaxCsvLimit, c.Publish, c.FeeRate,
	)
}

func sweepTimeLockManual(extendedKey *hdkeychain.ExtendedKey, apiURL string,
	sweepAddr, timeLockAddr string, remoteRevPoint *btcec.PublicKey,
	maxCsvTimeout int, publish bool, feeRate uint32) error {

	// First of all, we need to parse the lock addr and make sure we can
	// brute force the script with the information we have. If not, we can't
	// continue anyway.
	lockScript, err := lnd.GetP2WSHScript(timeLockAddr, chainParams)
	if err != nil {
		return fmt.Errorf("invalid time lock addr: %v", err)
	}

	// We need to go through a lot of our keys so it makes sense to
	// pre-derive the static part of our key path.
	basePath, err := lnd.ParsePath(fmt.Sprintf(
		keyBasePath, chainParams.HDCoinType,
	))
	if err != nil {
		return fmt.Errorf("could not derive base path: %v", err)
	}
	baseKey, err := lnd.DeriveChildren(extendedKey, basePath)
	if err != nil {
		return fmt.Errorf("could not derive base key: %v", err)
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
	for i := uint32(0); i < maxKeys; i++ {
		// The easy part first, let's derive the delay base point.
		delayPath := []uint32{
			lnd.HardenedKey(uint32(keychain.KeyFamilyDelayBase)), 0,
			i,
		}
		delayPrivKey, err := lnd.PrivKeyFromPath(baseKey, delayPath)
		if err != nil {
			return err
		}

		// Get the revocation base point first so we can calculate our
		// commit point.
		revPath := []uint32{
			lnd.HardenedKey(uint32(
				keychain.KeyFamilyRevocationRoot,
			)), 0, i,
		}
		revRoot, err := lnd.ShaChainFromPath(baseKey, revPath)
		if err != nil {
			return err
		}

		// We now have everything to brute force the lock script. This
		// will take a long while as we both have to go through commit
		// points and CSV values.
		csvTimeout, script, scriptHash, commitPoint, err =
			bruteForceDelayPoint(
				delayPrivKey.PubKey(), remoteRevPoint, revRoot,
				lockScript, maxCsvTimeout,
			)

		if err == nil {
			delayDesc = &keychain.KeyDescriptor{
				PubKey: delayPrivKey.PubKey(),
				KeyLocator: keychain.KeyLocator{
					Family: keychain.KeyFamilyDelayBase,
					Index:  i,
				},
			}
			break
		}

		if i != 0 && i%20 == 0 {
			fmt.Printf("Tried %d of %d keys.", i, maxKeys)
		}
	}

	// Did we find what we looked for or did we just exhaust all
	// possibilities?
	if script == nil || delayDesc == nil {
		return fmt.Errorf("target script not derived")
	}

	// Create signer and transaction template.
	signer := &lnd.Signer{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}
	api := &btc.ExplorerAPI{BaseURL: apiURL}

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
		return fmt.Errorf("error parsing tx hash: %v", err)
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
	var estimator input.TxWeightEstimator
	estimator.AddWitnessInput(input.ToLocalTimeoutWitnessSize)
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
	sigHashes := txscript.NewTxSigHashes(sweepTx)
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
		InputIndex: 0,
		SigHashes:  sigHashes,
		HashType:   txscript.SigHashAll,
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

func bruteForceDelayPoint(delayBase, revBase *btcec.PublicKey,
	revRoot *shachain.RevocationProducer, lockScript []byte,
	maxCsvTimeout int) (int32, []byte, []byte, *btcec.PublicKey, error) {

	for i := uint64(0); i < maxPoints; i++ {
		revPreimage, err := revRoot.AtIndex(i)
		if err != nil {
			return 0, nil, nil, nil, err
		}
		commitPoint := input.ComputeCommitmentPoint(revPreimage[:])

		csvTimeout, script, scriptHash, err := bruteForceDelay(
			input.TweakPubKey(delayBase, commitPoint),
			input.DeriveRevocationPubkey(revBase, commitPoint),
			lockScript, maxCsvTimeout,
		)

		if err != nil {
			continue
		}

		return csvTimeout, script, scriptHash, commitPoint, nil
	}

	return 0, nil, nil, nil, fmt.Errorf("target script not derived")
}
