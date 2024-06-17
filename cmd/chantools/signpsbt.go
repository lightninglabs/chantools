package main

import (
	"bytes"
	"encoding/base64"
	"encoding/binary"
	"errors"
	"fmt"
	"os"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/btcutil/psbt"
	"github.com/btcsuite/btcd/txscript"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/spf13/cobra"
)

var (
	errNoPathFound = fmt.Errorf("no matching derivation path found")
)

type signPSBTCommand struct {
	Psbt            string
	FromRawPsbtFile string
	ToRawPsbtFile   string

	rootKey *rootKey
	cmd     *cobra.Command
}

func newSignPSBTCommand() *cobra.Command {
	cc := &signPSBTCommand{}
	cc.cmd = &cobra.Command{
		Use:   "signpsbt",
		Short: "Sign a Partially Signed Bitcoin Transaction (PSBT)",
		Long: `Sign a PSBT with a master root key. The PSBT must contain
an input that is owned by the master root key.`,
		Example: `chantools signpsbt \
	--psbt <the_base64_encoded_psbt>

chantools signpsbt --fromrawpsbtfile <file_with_psbt>`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.Psbt, "psbt", "", "Partially Signed Bitcoin Transaction "+
			"to sign",
	)
	cc.cmd.Flags().StringVar(
		&cc.FromRawPsbtFile, "fromrawpsbtfile", "", "the file containing "+
			"the raw, binary encoded PSBT packet to sign",
	)
	cc.cmd.Flags().StringVar(
		&cc.ToRawPsbtFile, "torawpsbtfile", "", "the file to write "+
			"the resulting signed raw, binary encoded PSBT packet "+
			"to",
	)

	cc.rootKey = newRootKey(cc.cmd, "signing the PSBT")

	return cc.cmd
}

func (c *signPSBTCommand) Execute(_ *cobra.Command, _ []string) error {
	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %w", err)
	}

	signer := &lnd.Signer{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}

	var packet *psbt.Packet

	// Decode the PSBT, either from the command line or the binary file.
	switch {
	case c.Psbt != "":
		packet, err = psbt.NewFromRawBytes(
			bytes.NewReader([]byte(c.Psbt)), true,
		)
		if err != nil {
			return fmt.Errorf("error decoding PSBT: %w", err)
		}

	case c.FromRawPsbtFile != "":
		f, err := os.Open(c.FromRawPsbtFile)
		if err != nil {
			return fmt.Errorf("error opening PSBT file '%s': %w",
				c.FromRawPsbtFile, err)
		}

		packet, err = psbt.NewFromRawBytes(f, false)
		if err != nil {
			return fmt.Errorf("error decoding PSBT from file "+
				"'%s': %w", c.FromRawPsbtFile, err)
		}

	default:
		return fmt.Errorf("either the PSBT or the raw PSBT file " +
			"must be set")
	}

	err = signPsbt(extendedKey, packet, signer)
	if err != nil {
		return fmt.Errorf("error signing PSBT: %w", err)
	}

	switch {
	case c.ToRawPsbtFile != "":
		f, err := os.Create(c.ToRawPsbtFile)
		if err != nil {
			return fmt.Errorf("error creating PSBT file '%s': %w",
				c.ToRawPsbtFile, err)
		}

		if err := packet.Serialize(f); err != nil {
			return fmt.Errorf("error serializing PSBT to file "+
				"'%s': %w", c.ToRawPsbtFile, err)
		}

		fmt.Printf("Successfully signed PSBT and wrote it to file "+
			"'%s'\n", c.ToRawPsbtFile)

	default:
		var buf bytes.Buffer
		if err := packet.Serialize(&buf); err != nil {
			return fmt.Errorf("error serializing PSBT: %w", err)
		}

		fmt.Printf("Successfully signed PSBT:\n\n%s\n",
			base64.StdEncoding.EncodeToString(buf.Bytes()))
	}

	return nil
}

func signPsbt(rootKey *hdkeychain.ExtendedKey,
	packet *psbt.Packet, signer *lnd.Signer) error {

	for inputIndex := range packet.Inputs {
		pIn := &packet.Inputs[inputIndex]

		// Check that we have an input with a derivation path that
		// belongs to the root key.
		derivationPath, err := findMatchingDerivationPath(rootKey, pIn)
		if errors.Is(err, errNoPathFound) {
			log.Infof("No matching derivation path found for "+
				"input %d, skipping", inputIndex)
			continue
		}
		if err != nil {
			return fmt.Errorf("could not find matching derivation "+
				"path: %w", err)
		}

		if len(derivationPath) < 5 {
			return fmt.Errorf("invalid derivation path, expected "+
				"at least 5 elements, got %d",
				len(derivationPath))
		}

		localKey, err := lnd.DeriveChildren(rootKey, derivationPath)
		if err != nil {
			return fmt.Errorf("could not derive local key: %w", err)
		}

		if pIn.WitnessUtxo == nil {
			return fmt.Errorf("invalid PSBT, input %d is missing "+
				"witness UTXO", inputIndex)
		}
		utxo := pIn.WitnessUtxo

		// The signing is a bit different for P2WPKH, we need to specify
		// the pk script as the witness script.
		var witnessScript []byte
		if txscript.IsPayToWitnessPubKeyHash(utxo.PkScript) {
			witnessScript = utxo.PkScript
		} else {
			if len(pIn.WitnessScript) == 0 {
				return fmt.Errorf("invalid PSBT, input %d is "+
					"missing witness script", inputIndex)
			}
			witnessScript = pIn.WitnessScript
		}

		localPrivateKey, err := localKey.ECPrivKey()
		if err != nil {
			return fmt.Errorf("error getting private key: %w", err)
		}

		// Do we already have a partial signature for our key?
		localPubKey := localPrivateKey.PubKey().SerializeCompressed()
		haveSig := false
		for _, partialSig := range pIn.PartialSigs {
			if bytes.Equal(partialSig.PubKey, localPubKey) {
				haveSig = true
			}
		}
		if haveSig {
			log.Infof("Already have a partial signature for input "+
				"%d and local key %x, skipping", inputIndex,
				localPubKey)
			continue
		}

		err = signer.AddPartialSignatureForPrivateKey(
			packet, localPrivateKey, utxo, witnessScript,
			inputIndex,
		)
		if err != nil {
			return fmt.Errorf("error adding partial signature: %w",
				err)
		}
	}

	return nil
}

func findMatchingDerivationPath(rootKey *hdkeychain.ExtendedKey,
	pIn *psbt.PInput) ([]uint32, error) {

	pubKey, err := rootKey.ECPubKey()
	if err != nil {
		return nil, fmt.Errorf("error getting public key: %w", err)
	}

	pubKeyHash := btcutil.Hash160(pubKey.SerializeCompressed())
	fingerprint := binary.LittleEndian.Uint32(pubKeyHash[:4])

	if len(pIn.Bip32Derivation) == 0 {
		return nil, errNoPathFound
	}

	for _, derivation := range pIn.Bip32Derivation {
		// A special case where there is only a single derivation path
		// and the master key fingerprint is not set, we assume we are
		// the correct signer... This might not be correct, but we have
		// no way of knowing.
		if derivation.MasterKeyFingerprint == 0 &&
			len(pIn.Bip32Derivation) == 1 {

			return derivation.Bip32Path, nil
		}

		// The normal case, where a derivation path has the master
		// fingerprint set.
		if derivation.MasterKeyFingerprint == fingerprint {
			return derivation.Bip32Path, nil
		}
	}

	return nil, errNoPathFound
}
