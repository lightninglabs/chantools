package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"math"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/btcutil/psbt"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/mempool"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightninglabs/chantools/btc"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lnwallet/chainfee"
	"github.com/spf13/cobra"
)

type pullAnchorCommand struct {
	APIURL       string
	SponsorInput string
	AnchorAddrs  []string
	ChangeAddr   string
	FeeRate      uint32

	rootKey *rootKey
	cmd     *cobra.Command
}

func newPullAnchorCommand() *cobra.Command {
	cc := &pullAnchorCommand{}
	cc.cmd = &cobra.Command{
		Use:   "pullanchor",
		Short: "Attempt to CPFP an anchor output of a channel",
		Long: `Use this command to confirm a channel force close
transaction of an anchor output channel type. This will attempt to CPFP the
330 byte anchor output created for your node.`,
		Example: `chantools pullanchor \
	--sponsorinput txid:vout \
	--anchoraddr bc1q..... \
	--changeaddr bc1q..... \
	--feerate 30`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.APIURL, "apiurl", defaultAPIURL, "API URL to use (must "+
			"be esplora compatible)",
	)
	cc.cmd.Flags().StringVar(
		&cc.SponsorInput, "sponsorinput", "", "the input to use to "+
			"sponsor the CPFP transaction; must be owned by the "+
			"lnd node that owns the anchor output",
	)
	cc.cmd.Flags().StringArrayVar(
		&cc.AnchorAddrs, "anchoraddr", nil, "the address of the "+
			"anchor output (p2wsh or p2tr output with 330 "+
			"satoshis) that should be pulled; can be specified "+
			"multiple times per command to pull multiple anchors "+
			"with a single transaction",
	)
	cc.cmd.Flags().StringVar(
		&cc.ChangeAddr, "changeaddr", "", "the change address to "+
			"send the remaining funds to",
	)
	cc.cmd.Flags().Uint32Var(
		&cc.FeeRate, "feerate", defaultFeeSatPerVByte, "fee rate to "+
			"use for the sweep transaction in sat/vByte",
	)

	cc.rootKey = newRootKey(cc.cmd, "deriving keys")

	return cc.cmd
}

func (c *pullAnchorCommand) Execute(_ *cobra.Command, _ []string) error {
	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %w", err)
	}

	// Make sure all input is provided.
	if c.SponsorInput == "" {
		return fmt.Errorf("sponsor input is required")
	}
	if len(c.AnchorAddrs) == 0 {
		return fmt.Errorf("at least one anchor addr is required")
	}
	if c.ChangeAddr == "" {
		return fmt.Errorf("change addr is required")
	}

	outpoint, err := lnd.ParseOutpoint(c.SponsorInput)
	if err != nil {
		return fmt.Errorf("error parsing sponsor input outpoint: %w",
			err)
	}

	changeScript, err := lnd.GetP2WPKHScript(c.ChangeAddr, chainParams)
	if err != nil {
		return fmt.Errorf("error parsing change addr: %w", err)
	}

	// Set default values.
	if c.FeeRate == 0 {
		c.FeeRate = defaultFeeSatPerVByte
	}
	return createPullTransactionTemplate(
		extendedKey, c.APIURL, outpoint, c.AnchorAddrs, changeScript,
		c.FeeRate,
	)
}

type targetAnchor struct {
	addr       string
	keyDesc    *keychain.KeyDescriptor
	outpoint   wire.OutPoint
	utxo       *wire.TxOut
	script     []byte
	scriptTree *input.AnchorScriptTree
}

func createPullTransactionTemplate(rootKey *hdkeychain.ExtendedKey,
	apiURL string, sponsorOutpoint *wire.OutPoint, anchorAddrs []string,
	changeScript []byte, feeRate uint32) error {

	signer := &lnd.Signer{
		ExtendedKey: rootKey,
		ChainParams: chainParams,
	}
	api := &btc.ExplorerAPI{BaseURL: apiURL}
	estimator := input.TxWeightEstimator{}

	// Make sure the sponsor input is a P2WPKH or P2TR input and is known
	// to the block explorer, so we can fetch the witness utxo.
	sponsorTx, err := api.Transaction(sponsorOutpoint.Hash.String())
	if err != nil {
		return fmt.Errorf("error fetching sponsor tx: %w", err)
	}
	sponsorTxOut := sponsorTx.Vout[sponsorOutpoint.Index]
	sponsorPkScript, err := hex.DecodeString(sponsorTxOut.ScriptPubkey)
	if err != nil {
		return fmt.Errorf("error decoding sponsor pkscript: %w", err)
	}

	sponsorType, err := txscript.ParsePkScript(sponsorPkScript)
	if err != nil {
		return fmt.Errorf("error parsing sponsor pkscript: %w", err)
	}
	var sponsorSigHashType txscript.SigHashType
	switch sponsorType.Class() {
	case txscript.WitnessV0PubKeyHashTy:
		estimator.AddP2WKHInput()
		sponsorSigHashType = txscript.SigHashAll

	case txscript.WitnessV1TaprootTy:
		sponsorSigHashType = txscript.SigHashDefault
		estimator.AddTaprootKeySpendInput(sponsorSigHashType)

	default:
		return fmt.Errorf("unsupported sponsor input type: %v",
			sponsorType.Class())
	}

	tx := wire.NewMsgTx(2)
	packet, err := psbt.NewFromUnsignedTx(tx)
	if err != nil {
		return fmt.Errorf("error creating PSBT: %w", err)
	}

	// Let's add the sponsor input to the PSBT.
	sponsorUtxo := &wire.TxOut{
		Value:    int64(sponsorTxOut.Value),
		PkScript: sponsorPkScript,
	}
	packet.UnsignedTx.TxIn = append(packet.UnsignedTx.TxIn, &wire.TxIn{
		PreviousOutPoint: *sponsorOutpoint,
		Sequence:         mempool.MaxRBFSequence,
	})
	packet.Inputs = append(packet.Inputs, psbt.PInput{
		WitnessUtxo: sponsorUtxo,
		SighashType: sponsorSigHashType,
	})

	targets, err := addAnchorInputs(
		anchorAddrs, packet, api, &estimator, rootKey,
	)
	if err != nil {
		return fmt.Errorf("error adding anchor inputs: %w", err)
	}

	// Now we can calculate the fee and add the change output.
	estimator.AddP2WKHOutput()
	totalOutputValue := btcutil.Amount(sponsorTxOut.Value + 330)
	feeRateKWeight := chainfee.SatPerKVByte(1000 * feeRate).FeePerKWeight()
	totalFee := feeRateKWeight.FeeForWeight(int64(estimator.Weight()))

	log.Infof("Fee %d sats of %d total amount (estimated weight %d)",
		totalFee, totalOutputValue, estimator.Weight())

	packet.UnsignedTx.TxOut = append(packet.UnsignedTx.TxOut, &wire.TxOut{
		Value:    int64(totalOutputValue - totalFee),
		PkScript: changeScript,
	})
	packet.Outputs = append(packet.Outputs, psbt.POutput{})

	prevOutFetcher := txscript.NewMultiPrevOutFetcher(
		map[wire.OutPoint]*wire.TxOut{
			*sponsorOutpoint: sponsorUtxo,
		},
	)
	for idx := range targets {
		prevOutFetcher.AddPrevOut(
			targets[idx].outpoint, targets[idx].utxo,
		)
	}

	// And now we sign the anchor inputs.
	for idx := range targets {
		target := targets[idx]
		signDesc := &input.SignDescriptor{
			KeyDesc:           *target.keyDesc,
			WitnessScript:     target.script,
			Output:            target.utxo,
			PrevOutputFetcher: prevOutFetcher,
			InputIndex:        idx + 1,
		}

		var anchorWitness wire.TxWitness
		switch {
		// Simple Taproot Channel:
		case target.scriptTree != nil:
			signDesc.SignMethod = input.TaprootKeySpendSignMethod
			signDesc.HashType = txscript.SigHashDefault
			signDesc.TapTweak = target.scriptTree.TapscriptRoot

			anchorSig, err := signer.SignOutputRaw(
				packet.UnsignedTx, signDesc,
			)
			if err != nil {
				return fmt.Errorf("error signing anchor "+
					"input: %w", err)
			}

			anchorWitness = wire.TxWitness{
				anchorSig.Serialize(),
			}

		// Anchor Channel:
		default:
			signDesc.SignMethod = input.WitnessV0SignMethod
			signDesc.HashType = txscript.SigHashAll

			anchorSig, err := signer.SignOutputRaw(
				packet.UnsignedTx, signDesc,
			)
			if err != nil {
				return fmt.Errorf("error signing anchor "+
					"input: %w", err)
			}

			anchorWitness = make(wire.TxWitness, 2)
			anchorWitness[0] = append(
				anchorSig.Serialize(),
				byte(txscript.SigHashAll),
			)
			anchorWitness[1] = target.script
		}

		var witnessBuf bytes.Buffer
		err = psbt.WriteTxWitness(&witnessBuf, anchorWitness)
		if err != nil {
			return fmt.Errorf("error serializing witness: %w", err)
		}

		packet.Inputs[idx+1].FinalScriptWitness = witnessBuf.Bytes()
	}

	packetBase64, err := packet.B64Encode()
	if err != nil {
		return fmt.Errorf("error encoding PSBT: %w", err)
	}

	log.Infof("Prepared PSBT follows, please now call\n" +
		"'lncli wallet psbt finalize <psbt>' to finalize the\n" +
		"transaction, then publish it manually or by using\n" +
		"'lncli wallet publishtx <final_tx>':\n\n" + packetBase64 +
		"\n")

	return nil
}

func addAnchorInputs(anchorAddrs []string, packet *psbt.Packet,
	api *btc.ExplorerAPI, estimator *input.TxWeightEstimator,
	rootKey *hdkeychain.ExtendedKey) ([]targetAnchor, error) {

	// Fetch the additional info we need for the anchor output as well.
	results := make([]targetAnchor, len(anchorAddrs))
	for idx, anchorAddr := range anchorAddrs {
		anchorTx, anchorIndex, err := api.Outpoint(anchorAddr)
		if err != nil {
			return nil, fmt.Errorf("error fetching anchor "+
				"outpoint: %w", err)
		}
		anchorTxHash, err := chainhash.NewHashFromStr(anchorTx.TXID)
		if err != nil {
			return nil, fmt.Errorf("error decoding anchor txid: %w",
				err)
		}

		addr, err := btcutil.DecodeAddress(anchorAddr, chainParams)
		if err != nil {
			return nil, fmt.Errorf("error decoding address: %w",
				err)
		}

		anchorPkScript, err := txscript.PayToAddrScript(addr)
		if err != nil {
			return nil, fmt.Errorf("error creating pk script: %w",
				err)
		}

		target := targetAnchor{
			addr: anchorAddr,
			utxo: &wire.TxOut{
				Value:    330,
				PkScript: anchorPkScript,
			},
			outpoint: wire.OutPoint{
				Hash:  *anchorTxHash,
				Index: uint32(anchorIndex),
			},
		}
		switch addr.(type) {
		case *btcutil.AddressWitnessScriptHash:
			estimator.AddWitnessInput(input.AnchorWitnessSize)

			anchorKeyDesc, anchorWitnessScript, err := findAnchorKey(
				rootKey, anchorPkScript,
			)
			if err != nil {
				return nil, fmt.Errorf("could not find "+
					"key for anchor address %v: %w",
					anchorAddr, err)
			}

			target.keyDesc = anchorKeyDesc
			target.script = anchorWitnessScript

		case *btcutil.AddressTaproot:
			estimator.AddTaprootKeySpendInput(
				txscript.SigHashDefault,
			)

			anchorKeyDesc, scriptTree, err := findTaprootAnchorKey(
				rootKey, anchorPkScript,
			)
			if err != nil {
				return nil, fmt.Errorf("could not find "+
					"key for anchor address %v: %w",
					anchorAddr, err)
			}

			target.keyDesc = anchorKeyDesc
			target.scriptTree = scriptTree

		default:
			return nil, fmt.Errorf("unsupported address type: %T",
				addr)
		}

		log.Infof("Found multisig key %x for anchor pk script %x",
			target.keyDesc.PubKey.SerializeCompressed(),
			anchorPkScript)

		packet.UnsignedTx.TxIn = append(
			packet.UnsignedTx.TxIn, &wire.TxIn{
				PreviousOutPoint: target.outpoint,
				Sequence:         mempool.MaxRBFSequence,
			},
		)
		packet.Inputs = append(packet.Inputs, psbt.PInput{
			WitnessUtxo:   target.utxo,
			WitnessScript: target.script,
		})

		results[idx] = target
	}

	return results, nil
}

func findAnchorKey(rootKey *hdkeychain.ExtendedKey,
	targetScript []byte) (*keychain.KeyDescriptor, []byte, error) {

	family := keychain.KeyFamilyMultiSig
	localMultisig, err := lnd.DeriveChildren(rootKey, []uint32{
		lnd.HardenedKeyStart + uint32(keychain.BIP0043Purpose),
		lnd.HardenedKeyStart + chainParams.HDCoinType,
		lnd.HardenedKeyStart + uint32(family),
		0,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("could not derive local "+
			"multisig key: %w", err)
	}

	// Loop through the local multisig keys to find the target anchor
	// script.
	for index := uint32(0); index < math.MaxInt16; index++ {
		currentKey, err := localMultisig.DeriveNonStandard(index)
		if err != nil {
			return nil, nil, fmt.Errorf("error deriving child "+
				"key: %w", err)
		}

		currentPubkey, err := currentKey.ECPubKey()
		if err != nil {
			return nil, nil, fmt.Errorf("error deriving public "+
				"key: %w", err)
		}

		script, err := input.CommitScriptAnchor(currentPubkey)
		if err != nil {
			return nil, nil, fmt.Errorf("error deriving script: "+
				"%w", err)
		}

		pkScript, err := input.WitnessScriptHash(script)
		if err != nil {
			return nil, nil, fmt.Errorf("error deriving script "+
				"hash: %w", err)
		}

		if !bytes.Equal(pkScript, targetScript) {
			continue
		}

		return &keychain.KeyDescriptor{
			PubKey: currentPubkey,
			KeyLocator: keychain.KeyLocator{
				Family: family,
				Index:  index,
			},
		}, script, nil
	}

	return nil, nil, fmt.Errorf("no matching pubkeys found")
}

func findTaprootAnchorKey(rootKey *hdkeychain.ExtendedKey,
	targetScript []byte) (*keychain.KeyDescriptor, *input.AnchorScriptTree,
	error) {

	family := keychain.KeyFamilyPaymentBase
	localPayment, err := lnd.DeriveChildren(rootKey, []uint32{
		lnd.HardenedKeyStart + uint32(keychain.BIP0043Purpose),
		lnd.HardenedKeyStart + chainParams.HDCoinType,
		lnd.HardenedKeyStart + uint32(family),
		0,
	})
	if err != nil {
		return nil, nil, fmt.Errorf("could not derive local "+
			"multisig key: %w", err)
	}

	// Loop through the local multisig keys to find the target anchor
	// script.
	for index := uint32(0); index < math.MaxInt16; index++ {
		currentKey, err := localPayment.DeriveNonStandard(index)
		if err != nil {
			return nil, nil, fmt.Errorf("error deriving child "+
				"key: %w", err)
		}

		currentPubkey, err := currentKey.ECPubKey()
		if err != nil {
			return nil, nil, fmt.Errorf("error deriving public "+
				"key: %w", err)
		}

		scriptTree, err := input.NewAnchorScriptTree(currentPubkey)
		if err != nil {
			return nil, nil, fmt.Errorf("error deriving taproot "+
				"key: %w", err)
		}

		pkScript, err := input.PayToTaprootScript(scriptTree.TaprootKey)
		if err != nil {
			return nil, nil, fmt.Errorf("error deriving pk "+
				"script: %w", err)
		}
		if !bytes.Equal(pkScript, targetScript) {
			continue
		}

		return &keychain.KeyDescriptor{
			PubKey: currentPubkey,
			KeyLocator: keychain.KeyLocator{
				Family: family,
				Index:  index,
			},
		}, scriptTree, nil
	}

	return nil, nil, fmt.Errorf("no matching pubkeys found")
}
