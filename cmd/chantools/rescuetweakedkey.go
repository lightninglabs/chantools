package main

import (
	"bytes"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/spf13/cobra"
)

var (
	ErrAddrNotFound = fmt.Errorf("address not found")
)

type rescueTweakedKeyCommand struct {
	Path       string
	TargetAddr string
	NumTries   uint64

	rootKey *rootKey
	cmd     *cobra.Command
}

func newRescueTweakedKeyCommand() *cobra.Command {
	cc := &rescueTweakedKeyCommand{}
	cc.cmd = &cobra.Command{
		Use: "rescuetweakedkey",
		Short: "Attempt to rescue funds locked in an address with a " +
			"key that was affected by a specific bug in lnd",
		Long: `There very likely is no reason to run this command 
unless you exactly know why or were told by the author of this tool to use it.
`,
		Example: `chantools rescuetweakedkey \
	--path "m/1017'/0'/5'/0/0'" \
	--targetaddr bc1pxxxxxxx`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.Path, "path", "", "BIP32 derivation path to derive the "+
			"starting key from; must start with \"m/\"",
	)
	cc.cmd.Flags().StringVar(
		&cc.TargetAddr, "targetaddr", "", "address the funds are "+
			"locked in",
	)
	cc.cmd.Flags().Uint64Var(
		&cc.NumTries, "numtries", 10_000_000, "the number of "+
			"mutations to try",
	)

	cc.rootKey = newRootKey(cc.cmd, "deriving starting key")

	return cc.cmd
}

func (c *rescueTweakedKeyCommand) Execute(_ *cobra.Command, _ []string) error {
	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %w", err)
	}

	if c.Path == "" {
		return fmt.Errorf("path is required")
	}

	childKey, _, _, err := lnd.DeriveKey(extendedKey, c.Path, chainParams)
	if err != nil {
		return fmt.Errorf("could not derive key: %w", err)
	}

	startKey, err := childKey.ECPrivKey()
	if err != nil {
		return fmt.Errorf("error deriving private key: %w", err)
	}

	targetAddr, err := lnd.ParseAddress(c.TargetAddr, chainParams)
	if err != nil {
		return fmt.Errorf("error parsing target addr: %w", err)
	}

	return testPattern(startKey, targetAddr, c.NumTries)
}

func testPattern(startKey *btcec.PrivateKey, targetAddr btcutil.Address,
	max uint64) error {

	currentKey := copyPrivKey(startKey)
	for idx := uint64(0); idx <= max; idx++ {
		match, err := pubKeyMatchesAddr(currentKey.PubKey(), targetAddr)
		if err != nil {
			return fmt.Errorf("error matching key to address: %w",
				err)
		}

		if match {
			log.Infof("Success! Found private key %x for "+
				"address %v\n", currentKey.Serialize(),
				targetAddr)
			return nil
		}

		mutateWithTweak(currentKey)

		match, err = pubKeyMatchesAddr(currentKey.PubKey(), targetAddr)
		if err != nil {
			return fmt.Errorf("error matching key to address: %w",
				err)
		}

		if match {
			log.Infof("Success! Found private key %x for "+
				"address %v\n", currentKey.Serialize(),
				targetAddr)
			return nil
		}

		keyCopy := copyPrivKey(currentKey)
		mutateWithSign(keyCopy)

		match, err = pubKeyMatchesAddr(keyCopy.PubKey(), targetAddr)
		if err != nil {
			return fmt.Errorf("error matching key to address: %w",
				err)
		}

		if match {
			log.Infof("Success! Found private key %x for "+
				"address %v\n", keyCopy.Serialize(),
				targetAddr)
			return nil
		}

		if idx != 0 && idx%5000 == 0 {
			fmt.Printf("Tested %d of %d mutations\n", idx, max)
		}
	}

	match, err := pubKeyMatchesAddr(currentKey.PubKey(), targetAddr)
	if err != nil {
		return fmt.Errorf("error matching key to address: %w", err)
	}

	if match {
		log.Infof("Success! Found private key %x for address %v\n",
			currentKey.Serialize(), targetAddr)
		return nil
	}

	return fmt.Errorf("%w: key for address %v not found after %d attempts",
		ErrAddrNotFound, targetAddr.String(), max)
}

func pubKeyMatchesAddr(pubKey *btcec.PublicKey, addr btcutil.Address) (bool,
	error) {

	switch typedAddr := addr.(type) {
	case *btcutil.AddressWitnessPubKeyHash:
		hash160 := btcutil.Hash160(pubKey.SerializeCompressed())

		return bytes.Equal(hash160, typedAddr.WitnessProgram()), nil

	case *btcutil.AddressTaproot:
		taprootKey := txscript.ComputeTaprootKeyNoScript(pubKey)

		return bytes.Equal(
			schnorr.SerializePubKey(taprootKey),
			typedAddr.WitnessProgram(),
		), nil

	default:
		return false, fmt.Errorf("unsupported address type <%T>",
			typedAddr)
	}
}

func copyPrivKey(privKey *btcec.PrivateKey) *btcec.PrivateKey {
	privKeyCopy := *privKey
	return &btcec.PrivateKey{
		Key: privKeyCopy.Key,
	}
}

func mutateWithSign(privKey *btcec.PrivateKey) {
	privKeyScalar := &privKey.Key
	pub := privKey.PubKey()

	// Step 5.
	//
	// Negate d if P.y is odd.
	pubKeyBytes := pub.SerializeCompressed()
	if pubKeyBytes[0] == secp256k1.PubKeyFormatCompressedOdd {
		privKeyScalar.Negate()
	}
}

func mutateWithTweak(privKey *btcec.PrivateKey) {
	// If the corresponding public key has an odd y coordinate, then we'll
	// negate the private key as specified in BIP 341.
	privKeyScalar := &privKey.Key
	pubKeyBytes := privKey.PubKey().SerializeCompressed()
	if pubKeyBytes[0] == secp256k1.PubKeyFormatCompressedOdd {
		privKeyScalar.Negate()
	}

	// Next, we'll compute the tap tweak hash that commits to the internal
	// key and the merkle script root. We'll snip off the extra parity byte
	// from the compressed serialization and use that directly.
	schnorrKeyBytes := pubKeyBytes[1:]
	tapTweakHash := chainhash.TaggedHash(
		chainhash.TagTapTweak, schnorrKeyBytes, []byte{},
	)

	// Map the private key to a ModNScalar which is needed to perform
	// operation mod the curve order.
	var tweakScalar btcec.ModNScalar
	tweakScalar.SetBytes((*[32]byte)(tapTweakHash))

	// Now that we have the private key in its may negated form, we'll add
	// the script root as a tweak. As we're using a ModNScalar all
	// operations are already normalized mod the curve order.
	_ = privKeyScalar.Add(&tweakScalar)
}
