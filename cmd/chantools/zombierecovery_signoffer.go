package main

import (
	"bufio"
	"bytes"
	"fmt"
	"os"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/btcutil/psbt"
	"github.com/btcsuite/btcd/txscript"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/spf13/cobra"
)

type zombieRecoverySignOfferCommand struct {
	Psbt string

	rootKey *rootKey
	cmd     *cobra.Command
}

func newZombieRecoverySignOfferCommand() *cobra.Command {
	cc := &zombieRecoverySignOfferCommand{}
	cc.cmd = &cobra.Command{
		Use: "signoffer",
		Short: "[3/3] Sign an offer sent by the remote peer to " +
			"recover funds",
		Long: `Inspect and sign an offer that was sent by the remote
peer to recover funds from one or more channels.`,
		Example: `chantools zombierecovery signoffer \
	--psbt <offered_psbt_base64>`,
		RunE: cc.Execute,
	}

	cc.cmd.Flags().StringVar(
		&cc.Psbt, "psbt", "", "the base64 encoded PSBT that the other "+
			"party sent as an offer to rescue funds",
	)

	cc.rootKey = newRootKey(cc.cmd, "signing the offer")

	return cc.cmd
}

func (c *zombieRecoverySignOfferCommand) Execute(_ *cobra.Command,
	_ []string) error {

	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %w", err)
	}

	signer := &lnd.Signer{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}

	// Decode the PSBT.
	packet, err := psbt.NewFromRawBytes(
		bytes.NewReader([]byte(c.Psbt)), true,
	)
	if err != nil {
		return fmt.Errorf("error decoding PSBT: %w", err)
	}

	return signOffer(extendedKey, packet, signer)
}

func signOffer(rootKey *hdkeychain.ExtendedKey,
	packet *psbt.Packet, signer *lnd.Signer) error {

	// First, we need to derive the correct branch from the local root key.
	localMultisig, err := lnd.DeriveChildren(rootKey, []uint32{
		lnd.HardenedKeyStart + uint32(keychain.BIP0043Purpose),
		lnd.HardenedKeyStart + chainParams.HDCoinType,
		lnd.HardenedKeyStart + uint32(keychain.KeyFamilyMultiSig),
		0,
	})
	if err != nil {
		return fmt.Errorf("could not derive local multisig key: %w",
			err)
	}

	// Now let's check that the packet has the expected proprietary key with
	// our pubkey that we need to sign with.
	if len(packet.Inputs) == 0 {
		return fmt.Errorf("invalid PSBT, expected at least 1 input, "+
			"got %d", len(packet.Inputs))
	}
	for idx := range packet.Inputs {
		if len(packet.Inputs[idx].Unknowns) != 1 {
			return fmt.Errorf("invalid PSBT, expected 1 unknown "+
				"in input %d, got %d", idx,
				len(packet.Inputs[idx].Unknowns))
		}
	}

	fmt.Printf("The PSBT contains the following proposal:\n\n\t"+
		"Close %d channels: \n", len(packet.Inputs))
	var totalInput int64
	for idx, txIn := range packet.UnsignedTx.TxIn {
		value := packet.Inputs[idx].WitnessUtxo.Value
		totalInput += value
		fmt.Printf("\tChannel %d (%s:%d), capacity %d sats\n",
			idx, txIn.PreviousOutPoint.Hash.String(),
			txIn.PreviousOutPoint.Index, value)
	}
	fmt.Println()
	var totalOutput int64
	for _, txOut := range packet.UnsignedTx.TxOut {
		totalOutput += txOut.Value
		pkScript, err := txscript.ParsePkScript(txOut.PkScript)
		if err != nil {
			return fmt.Errorf("error parsing pk script: %w", err)
		}
		addr, err := pkScript.Address(chainParams)
		if err != nil {
			return fmt.Errorf("error parsing address: %w", err)
		}
		fmt.Printf("\tSend %d sats to address %s\n", txOut.Value, addr)
	}
	fmt.Printf("\n\tTotal fees: %d sats\n\nDo you want to continue?\n",
		totalInput-totalOutput)
	fmt.Printf("Press <enter> to continue and sign the transaction or " +
		"<ctrl+c> to abort: ")
	_, _ = bufio.NewReader(os.Stdin).ReadString('\n')

	for idx := range packet.Inputs {
		unknown := packet.Inputs[idx].Unknowns[0]
		if !bytes.Equal(unknown.Key, PsbtKeyTypeOutputMissingSigPubkey) {
			return fmt.Errorf("invalid PSBT, unknown has invalid "+
				"key %x, expected %x", unknown.Key,
				PsbtKeyTypeOutputMissingSigPubkey)
		}
		targetKey, err := btcec.ParsePubKey(unknown.Value)
		if err != nil {
			return fmt.Errorf("invalid PSBT, proprietary key has "+
				"invalid pubkey: %w", err)
		}

		// Now we can look up the local key and check the PSBT further,
		// then add our signature.
		localKeyDesc, err := findLocalMultisigKey(
			localMultisig, targetKey,
		)
		if err != nil {
			return fmt.Errorf("could not find local multisig key: "+
				"%w", err)
		}
		if len(packet.Inputs[idx].WitnessScript) == 0 {
			return fmt.Errorf("invalid PSBT, missing witness " +
				"script")
		}
		witnessScript := packet.Inputs[idx].WitnessScript
		if packet.Inputs[idx].WitnessUtxo == nil {
			return fmt.Errorf("invalid PSBT, witness UTXO missing")
		}
		utxo := packet.Inputs[idx].WitnessUtxo

		err = signer.AddPartialSignature(
			packet, *localKeyDesc, utxo, witnessScript, idx,
		)
		if err != nil {
			return fmt.Errorf("error adding partial signature: %w",
				err)
		}
	}

	// We're almost done. Now we just need to make sure we can finalize and
	// extract the final TX.
	err = psbt.MaybeFinalizeAll(packet)
	if err != nil {
		return fmt.Errorf("error finalizing PSBT: %w", err)
	}
	finalTx, err := psbt.Extract(packet)
	if err != nil {
		return fmt.Errorf("unable to extract final TX: %w", err)
	}
	var buf bytes.Buffer
	err = finalTx.Serialize(&buf)
	if err != nil {
		return fmt.Errorf("unable to serialize final TX: %w", err)
	}

	fmt.Printf("Success, we counter signed the PSBT and extracted the "+
		"final\ntransaction. Please publish this using any bitcoin "+
		"node:\n\n%x\n\n", buf.Bytes())

	return nil
}
