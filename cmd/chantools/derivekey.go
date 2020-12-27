package main

import (
	"fmt"

	"github.com/btcsuite/btcutil"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/guggero/chantools/lnd"
	"github.com/spf13/cobra"
)

type deriveKeyCommand struct {
	BIP39  bool
	Path   string
	Neuter bool

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
		Example: `chantools derivekey --rootkey xprvxxxxxxxxxx \
	--path "m/1017'/0'/5'/0/0'" --neuter`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().BoolVar(
		&cc.BIP39, "bip39", false, "read a classic BIP39 seed and "+
			"passphrase from the terminal instead of asking for "+
			"lnd seed format or providing the --rootkey flag",
	)
	cc.cmd.Flags().StringVar(
		&cc.Path, "path", "", "BIP32 derivation path to derive; must "+
			"start with \"m/\"",
	)
	cc.cmd.Flags().BoolVar(
		&cc.Neuter, "neuter", false, "don't output private key(s), "+
			"only public key(s)",
	)

	cc.rootKey = newRootKey(cc.cmd, "decrypting the backup")

	return cc.cmd
}

func (c *deriveKeyCommand) Execute(_ *cobra.Command, _ []string) error {
	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %v", err)
	}

	return deriveKey(extendedKey, c.Path, c.Neuter)
}

func deriveKey(extendedKey *hdkeychain.ExtendedKey, path string,
	neuter bool) error {

	fmt.Printf("Deriving path %s for network %s.\n", path, chainParams.Name)
	child, pubKey, wif, err := lnd.DeriveKey(extendedKey, path, chainParams)
	if err != nil {
		return fmt.Errorf("could not derive keys: %v", err)
	}
	neutered, err := child.Neuter()
	if err != nil {
		return fmt.Errorf("could not neuter child key: %v", err)
	}
	fmt.Printf("\nPublic key: %x\n", pubKey.SerializeCompressed())
	fmt.Printf("Extended public key (xpub): %s\n", neutered.String())

	// Print the address too.
	hash160 := btcutil.Hash160(pubKey.SerializeCompressed())
	addrP2PKH, err := btcutil.NewAddressPubKeyHash(hash160, chainParams)
	if err != nil {
		return fmt.Errorf("could not create address: %v", err)
	}
	addrP2WKH, err := btcutil.NewAddressWitnessPubKeyHash(
		hash160, chainParams,
	)
	if err != nil {
		return fmt.Errorf("could not create address: %v", err)
	}
	fmt.Printf("Address: %s\n", addrP2WKH)
	fmt.Printf("Legacy address: %s\n", addrP2PKH)

	if !neuter {
		fmt.Printf("\nPrivate key (WIF): %s\n", wif.String())
		fmt.Printf("Extended private key (xprv): %s\n", child.String())
	}

	return nil
}
