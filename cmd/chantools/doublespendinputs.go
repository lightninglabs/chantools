package main

import (
	"bytes"
	"encoding/hex"
	"fmt"
	"strconv"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lightninglabs/chantools/btc"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/lnwallet/chainfee"
	"github.com/spf13/cobra"
)

type doubleSpendInputs struct {
	APIURL         string
	InputOutpoints []string
	Publish        bool
	SweepAddr      string
	FeeRate        uint32
	RecoveryWindow uint32

	rootKey *rootKey
	cmd     *cobra.Command
}

func newDoubleSpendInputsCommand() *cobra.Command {
	cc := &doubleSpendInputs{}
	cc.cmd = &cobra.Command{
		Use: "doublespendinputs",
		Short: "Tries to double spend the given inputs by deriving the " +
			"private for the address and sweeping the funds to the given " +
			"address. This can only be used with inputs that belong to " +
			"an lnd wallet.",
		Example: `chantools doublespendinputs \
		--inputoutpoints xxxxxxxxx:y,xxxxxxxxx:y \
		--sweepaddr bc1q..... \
		--feerate 10 \
		--publish`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.APIURL, "apiurl", defaultAPIURL, "API URL to use (must "+
			"be esplora compatible)",
	)
	cc.cmd.Flags().StringSliceVar(
		&cc.InputOutpoints, "inputoutpoints", []string{},
		"list of outpoints to double spend in the format txid:vout",
	)
	cc.cmd.Flags().StringVar(
		&cc.SweepAddr, "sweepaddr", "", "address to sweep the funds to",
	)
	cc.cmd.Flags().Uint32Var(
		&cc.FeeRate, "feerate", defaultFeeSatPerVByte, "fee rate to "+
			"use for the sweep transaction in sat/vByte",
	)
	cc.cmd.Flags().Uint32Var(
		&cc.RecoveryWindow, "recoverywindow", defaultRecoveryWindow,
		"number of keys to scan per internal/external branch; output "+
			"will consist of double this amount of keys",
	)
	cc.cmd.Flags().BoolVar(
		&cc.Publish, "publish", false, "publish replacement TX to "+
			"the chain API instead of just printing the TX",
	)

	cc.rootKey = newRootKey(cc.cmd, "deriving the input keys")

	return cc.cmd
}

func (c *doubleSpendInputs) Execute(_ *cobra.Command, _ []string) error {
	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %w", err)
	}

	// Make sure sweep addr is set.
	if c.SweepAddr == "" {
		return fmt.Errorf("sweep addr is required")
	}

	// Make sure we have at least one input.
	if len(c.InputOutpoints) == 0 {
		return fmt.Errorf("inputoutpoints are required")
	}

	api := &btc.ExplorerAPI{BaseURL: c.APIURL}

	addresses := make([]btcutil.Address, 0, len(c.InputOutpoints))
	outpoints := make([]*wire.OutPoint, 0, len(c.InputOutpoints))
	privKeys := make([]*secp256k1.PrivateKey, 0, len(c.InputOutpoints))

	// Get the addresses for the inputs.
	for _, input := range c.InputOutpoints {
		addrString, err := api.Address(input)
		if err != nil {
			return err
		}

		addr, err := btcutil.DecodeAddress(addrString, chainParams)
		if err != nil {
			return err
		}

		addresses = append(addresses, addr)

		txHash, err := chainhash.NewHashFromStr(input[:64])
		if err != nil {
			return err
		}

		vout, err := strconv.Atoi(input[65:])
		if err != nil {
			return err
		}
		outpoint := wire.NewOutPoint(txHash, uint32(vout))

		outpoints = append(outpoints, outpoint)
	}

	// Create the paths for the addresses.
	p2wkhPath, err := lnd.ParsePath(lnd.WalletDefaultDerivationPath)
	if err != nil {
		return err
	}

	p2trPath, err := lnd.ParsePath(lnd.WalletBIP86DerivationPath)
	if err != nil {
		return err
	}

	// Start with the txweight estimator.
	estimator := input.TxWeightEstimator{}

	// Find the key for the given addresses and add their
	// output weight to the tx estimator.
	for _, addr := range addresses {
		var key *hdkeychain.ExtendedKey
		switch addr.(type) {
		case *btcutil.AddressWitnessPubKeyHash:
			key, err = iterateOverPath(
				extendedKey, addr, p2wkhPath, c.RecoveryWindow,
			)
			if err != nil {
				return err
			}

			estimator.AddP2WKHInput()

		case *btcutil.AddressTaproot:
			key, err = iterateOverPath(
				extendedKey, addr, p2trPath, c.RecoveryWindow,
			)
			if err != nil {
				return err
			}

			estimator.AddTaprootKeySpendInput(txscript.SigHashDefault)

		default:
			return fmt.Errorf("address type %T not supported", addr)
		}

		// Get the private key.
		privKey, err := key.ECPrivKey()
		if err != nil {
			return err
		}

		privKeys = append(privKeys, privKey)
	}

	// Now that we have the keys, we can create the transaction.
	prevOuts := make(map[wire.OutPoint]*wire.TxOut)

	// Next get the full value of the inputs.
	var totalInput btcutil.Amount
	for _, input := range outpoints {
		// Get the transaction.
		tx, err := api.Transaction(input.Hash.String())
		if err != nil {
			return err
		}

		value := tx.Vout[input.Index].Value

		// Get the output index.
		totalInput += btcutil.Amount(value)

		scriptPubkey, err := hex.DecodeString(tx.Vout[input.Index].ScriptPubkey)
		if err != nil {
			return err
		}

		// Add the output to the map.
		prevOuts[*input] = &wire.TxOut{
			Value:    int64(value),
			PkScript: scriptPubkey,
		}
	}

	// Calculate the fee.
	sweepAddr, err := btcutil.DecodeAddress(c.SweepAddr, chainParams)
	if err != nil {
		return err
	}

	switch sweepAddr.(type) {
	case *btcutil.AddressWitnessPubKeyHash:
		estimator.AddP2WKHOutput()

	case *btcutil.AddressTaproot:
		estimator.AddP2TROutput()

	default:
		return fmt.Errorf("address type %T not supported", sweepAddr)
	}

	// Calculate the fee.
	feeRateKWeight := chainfee.SatPerKVByte(1000 * c.FeeRate).FeePerKWeight()
	totalFee := feeRateKWeight.FeeForWeight(int64(estimator.Weight()))

	// Create the transaction.
	tx := wire.NewMsgTx(2)

	// Add the inputs.
	for _, input := range outpoints {
		tx.AddTxIn(wire.NewTxIn(input, nil, nil))
	}

	// Add the output.
	sweepScript, err := txscript.PayToAddrScript(sweepAddr)
	if err != nil {
		return err
	}

	tx.AddTxOut(wire.NewTxOut(int64(totalInput-totalFee), sweepScript))

	// Calculate the signature hash.
	prevOutFetcher := txscript.NewMultiPrevOutFetcher(prevOuts)
	sigHashes := txscript.NewTxSigHashes(tx, prevOutFetcher)

	// Sign the inputs depending on the address type.
	for i, outpoint := range outpoints {
		switch addresses[i].(type) {
		case *btcutil.AddressWitnessPubKeyHash:
			witness, err := txscript.WitnessSignature(
				tx, sigHashes, i, prevOuts[*outpoint].Value,
				prevOuts[*outpoint].PkScript,
				txscript.SigHashAll, privKeys[i], true,
			)
			if err != nil {
				return err
			}

			tx.TxIn[i].Witness = witness

		case *btcutil.AddressTaproot:
			rawTxSig, err := txscript.RawTxInTaprootSignature(
				tx, sigHashes, i,
				prevOuts[*outpoint].Value,
				prevOuts[*outpoint].PkScript,
				[]byte{}, txscript.SigHashDefault, privKeys[i],
			)
			if err != nil {
				return err
			}

			tx.TxIn[i].Witness = wire.TxWitness{
				rawTxSig,
			}

		default:
			return fmt.Errorf("address type %T not supported", addresses[i])
		}
	}

	// Serialize the transaction.
	var txBuf bytes.Buffer
	if err := tx.Serialize(&txBuf); err != nil {
		return err
	}

	// Print the transaction.
	fmt.Printf("Sweeping transaction:\n%s\n", hex.EncodeToString(txBuf.Bytes()))

	// Publish the transaction.
	if c.Publish {
		txid, err := api.PublishTx(hex.EncodeToString(txBuf.Bytes()))
		if err != nil {
			return err
		}

		fmt.Printf("Published transaction with txid %s\n", txid)
	}

	return nil
}

// iterateOverPath iterates over the given key path and tries to find the
// private key that corresponds to the given address.
func iterateOverPath(baseKey *hdkeychain.ExtendedKey, addr btcutil.Address,
	path []uint32, maxTries uint32) (*hdkeychain.ExtendedKey, error) {

	for i := uint32(0); i < maxTries; i++ {
		// Check for both the external and internal branch.
		for _, branch := range []uint32{0, 1} {
			// Create the path to derive the key.
			addrPath := append(path, branch, i) //nolint:gocritic

			// Derive the key.
			derivedKey, err := lnd.DeriveChildren(baseKey, addrPath)
			if err != nil {
				return nil, err
			}

			var address btcutil.Address
			switch addr.(type) {
			case *btcutil.AddressWitnessPubKeyHash:
				// Get the address for the derived key.
				derivedAddr, err := derivedKey.Address(chainParams)
				if err != nil {
					return nil, err
				}

				address, err = btcutil.NewAddressWitnessPubKeyHash(
					derivedAddr.ScriptAddress(), chainParams,
				)
				if err != nil {
					return nil, err
				}

			case *btcutil.AddressTaproot:

				pubkey, err := derivedKey.ECPubKey()
				if err != nil {
					return nil, err
				}

				pubkey = txscript.ComputeTaprootKeyNoScript(pubkey)

				address, err = btcutil.NewAddressTaproot(
					schnorr.SerializePubKey(pubkey), chainParams,
				)
				if err != nil {
					return nil, err
				}
			}

			// Compare the addresses.
			if address.String() == addr.String() {
				return derivedKey, nil
			}
		}
	}

	return nil, fmt.Errorf("could not find key for address %s", addr.String())
}
