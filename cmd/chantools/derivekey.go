package main

import (
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/lightninglabs/chantools/cln"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/spf13/cobra"
)

const deriveKeyFormat = `
Path:				%s
Network: 			%s
Master Fingerprint:             %x
Public key: 			%x
Extended public key (xpub): 	%v
Address: 			%v
Legacy address: 		%v
Taproot address:                %v
Private key (WIF): 		%s
Extended private key (xprv):	%s
`

type deriveKeyCommand struct {
	Path     string
	Neuter   bool
	Identity bool

	HsmSecret string

	rootKey *rootKey
	cmd     *cobra.Command
}

func newDeriveKeyCommand() *cobra.Command {
	cc := &deriveKeyCommand{}
	cc.cmd = &cobra.Command{
		Use:   "derivekey",
		Short: "Derive a key with a specific derivation path",
		Long: `This command derives a single key with the given BIP32
derivation path from the root key and prints it to the console.`,
		Example: `chantools derivekey --path "m/1017'/0'/5'/0/0'" \
	--neuter

chantools derivekey --identity`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.Path, "path", "", "BIP32 derivation path to derive; must "+
			"start with \"m/\"",
	)
	cc.cmd.Flags().BoolVar(
		&cc.Neuter, "neuter", false, "don't output private key(s), "+
			"only public key(s)",
	)
	cc.cmd.Flags().BoolVar(
		&cc.Identity, "identity", false, "derive the node's identity "+
			"public key",
	)
	cc.cmd.Flags().StringVar(
		&cc.HsmSecret, "hsm_secret", "", "the hex encoded HSM secret "+
			"to use for deriving the multisig keys for a CLN "+
			"node; obtain by running 'xxd -p -c32 "+
			"~/.lightning/bitcoin/hsm_secret'",
	)

	cc.rootKey = newRootKey(cc.cmd, "decrypting the backup")

	return cc.cmd
}

func (c *deriveKeyCommand) Execute(_ *cobra.Command, _ []string) error {
	if c.HsmSecret != "" {
		if c.Path != "" {
			return errors.New("cannot specify --path with " +
				"--hsm_secret, only identity key can be " +
				"derived")
		}

		secretBytes, err := hex.DecodeString(c.HsmSecret)
		if err != nil {
			return fmt.Errorf("error decoding HSM secret: %w", err)
		}

		var hsmSecret [32]byte
		copy(hsmSecret[:], secretBytes)

		nodePubKey, _, err := cln.NodeKey(hsmSecret)
		if err != nil {
			return fmt.Errorf("error deriving node key from HSM: "+
				"%w", err)
		}

		result := fmt.Sprintf(
			"Node identity public key: 	%x",
			nodePubKey.SerializeCompressed(),
		)
		fmt.Println(result)

		// For the tests, also log as trace level which is disabled by
		// default.
		log.Tracef(result)

		return nil
	}

	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %w", err)
	}

	if c.Identity {
		c.Path = lnd.IdentityPath(chainParams)
		c.Neuter = true
	}

	return deriveKey(extendedKey, c.Path, c.Neuter)
}

func deriveKey(extendedKey *hdkeychain.ExtendedKey, path string,
	neuter bool) error {

	child, pubKey, wif, err := lnd.DeriveKey(extendedKey, path, chainParams)
	if err != nil {
		return fmt.Errorf("could not derive keys: %w", err)
	}
	neutered, err := child.Neuter()
	if err != nil {
		return fmt.Errorf("could not neuter child key: %w", err)
	}

	// Print the address too.
	hash160 := btcutil.Hash160(pubKey.SerializeCompressed())
	addrP2PKH, err := btcutil.NewAddressPubKeyHash(hash160, chainParams)
	if err != nil {
		return fmt.Errorf("could not create address: %w", err)
	}
	addrP2WKH, err := btcutil.NewAddressWitnessPubKeyHash(
		hash160, chainParams,
	)
	if err != nil {
		return fmt.Errorf("could not create address: %w", err)
	}

	addrP2TR, err := lnd.P2TRAddr(pubKey, chainParams)
	if err != nil {
		return fmt.Errorf("could not create address: %w", err)
	}

	privKey, xPriv := na, na
	if !neuter && wif != nil {
		privKey, xPriv = wif.String(), child.String()
	}

	_, fingerPrintBytes, err := fingerprint(extendedKey)
	if err != nil {
		return fmt.Errorf("could not get fingerprint: %w", err)
	}

	result := fmt.Sprintf(
		deriveKeyFormat, path, chainParams.Name, fingerPrintBytes,
		pubKey.SerializeCompressed(), neutered, addrP2WKH, addrP2PKH,
		addrP2TR, privKey, xPriv,
	)
	fmt.Println(result)

	// For the tests, also log as trace level which is disabled by default.
	log.Tracef(result)

	return nil
}
