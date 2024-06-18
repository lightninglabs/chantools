package main

import (
	"errors"
	"fmt"

	chantools_lnd "github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/spf13/cobra"
	"github.com/tv42/zbase32"
)

var (
	signedMsgPrefix = []byte("Lightning Signed Message:")
)

type signMessageCommand struct {
	Msg string

	rootKey *rootKey
	cmd     *cobra.Command
}

func newSignMessageCommand() *cobra.Command {
	cc := &signMessageCommand{}
	cc.cmd = &cobra.Command{
		Use:   "signmessage",
		Short: "Sign a message with the node's private key.",
		Long: `Sign msg with the resident node's private key.
		Returns the signature as a zbase32 string.`,
		Example: `chantools signmessage --msg=foobar`,
		RunE:    cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.Msg, "msg", "", "the message to sign",
	)

	cc.rootKey = newRootKey(cc.cmd, "decrypting the backup")

	return cc.cmd
}

func (c *signMessageCommand) Execute(_ *cobra.Command, _ []string) error {
	if c.Msg == "" {
		return errors.New("please enter a valid msg")
	}

	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %w", err)
	}

	signer := &chantools_lnd.Signer{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}

	// Create the key locator for the node key.
	keyLocator := keychain.KeyLocator{
		Family: keychain.KeyFamilyNodeKey,
		Index:  0,
	}

	// Fetch the private key for node key.
	privKey, err := signer.FetchPrivateKey(&keychain.KeyDescriptor{
		KeyLocator: keyLocator,
	})
	if err != nil {
		return err
	}

	// Create a new signer.
	privKeyMsgSigner := keychain.NewPrivKeyMessageSigner(
		privKey, keyLocator,
	)

	// Prepend the special lnd prefix.
	// See: https://github.com/lightningnetwork/lnd/blob/63e698ec4990e678089533561fd95cfd684b67db/rpcserver.go#L1576 .
	msg := []byte(c.Msg)
	msg = append(signedMsgPrefix, msg...)
	sigBytes, err := privKeyMsgSigner.SignMessageCompact(msg, true)
	if err != nil {
		return err
	}

	// Encode the signature.
	sig := zbase32.EncodeToString(sigBytes)
	fmt.Println(sig)

	return nil
}
