package main

import (
	"bytes"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/btcutil/psbt"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/spf13/cobra"
)

type signRescueFundingCommand struct {
	Psbt string

	rootKey *rootKey
	cmd     *cobra.Command
}

func newSignRescueFundingCommand() *cobra.Command {
	cc := &signRescueFundingCommand{}
	cc.cmd = &cobra.Command{
		Use: "signrescuefunding",
		Short: "Rescue funds locked in a funding multisig output that " +
			"never resulted in a proper channel; this is the " +
			"command the remote node (the non-initiator) of the " +
			"channel needs to run",
		Long: `This is part 2 of a two phase process to rescue a channel
funding output that was created on chain by accident but never resulted in a
proper channel and no commitment transactions exist to spend the funds locked in
the 2-of-2 multisig.

If successful, this will create a final on-chain transaction that can be
broadcast by any Bitcoin node.`,
		Example: `chantools signrescuefunding \
	--psbt <the_base64_encoded_psbt_from_step_1>`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().StringVar(
		&cc.Psbt, "psbt", "", "Partially Signed Bitcoin Transaction "+
			"that was provided by the initiator of the channel to "+
			"rescue",
	)

	cc.rootKey = newRootKey(cc.cmd, "deriving keys")

	return cc.cmd
}

func (c *signRescueFundingCommand) Execute(_ *cobra.Command, _ []string) error {
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

	return signRescueFunding(extendedKey, packet, signer)
}

func signRescueFunding(rootKey *hdkeychain.ExtendedKey,
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
	if len(packet.Inputs) != 1 {
		return fmt.Errorf("invalid PSBT, expected 1 input, got %d",
			len(packet.Inputs))
	}
	if len(packet.Inputs[0].Unknowns) != 1 {
		return fmt.Errorf("invalid PSBT, expected 1 unknown in input, "+
			"got %d", len(packet.Inputs[0].Unknowns))
	}
	unknown := packet.Inputs[0].Unknowns[0]
	if !bytes.Equal(unknown.Key, PsbtKeyTypeOutputMissingSigPubkey) {
		return fmt.Errorf("invalid PSBT, unknown has invalid key %x, "+
			"expected %x", unknown.Key,
			PsbtKeyTypeOutputMissingSigPubkey)
	}
	targetKey, err := btcec.ParsePubKey(unknown.Value)
	if err != nil {
		return fmt.Errorf("invalid PSBT, proprietary key has invalid "+
			"pubkey: %w", err)
	}

	// Now we can look up the local key and check the PSBT further, then
	// add our signature.
	localKeyDesc, err := findLocalMultisigKey(localMultisig, targetKey)
	if err != nil {
		return fmt.Errorf("could not find local multisig key: %w", err)
	}
	if len(packet.Inputs[0].WitnessScript) == 0 {
		return fmt.Errorf("invalid PSBT, missing witness script")
	}
	witnessScript := packet.Inputs[0].WitnessScript
	if packet.Inputs[0].WitnessUtxo == nil {
		return fmt.Errorf("invalid PSBT, witness UTXO missing")
	}
	utxo := packet.Inputs[0].WitnessUtxo

	err = signer.AddPartialSignature(
		packet, *localKeyDesc, utxo, witnessScript, 0,
	)
	if err != nil {
		return fmt.Errorf("error adding partial signature: %w", err)
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

func findLocalMultisigKey(multisigBranch *hdkeychain.ExtendedKey,
	targetPubkey *btcec.PublicKey) (*keychain.KeyDescriptor, error) {

	// Loop through the local multisig keys to find the target key.
	for index := uint32(0); index < MaxChannelLookup; index++ {
		currentKey, err := multisigBranch.DeriveNonStandard(index)
		if err != nil {
			return nil, fmt.Errorf("error deriving child key: %w",
				err)
		}

		currentPubkey, err := currentKey.ECPubKey()
		if err != nil {
			return nil, fmt.Errorf("error deriving public key: %w",
				err)
		}

		if !targetPubkey.IsEqual(currentPubkey) {
			continue
		}

		return &keychain.KeyDescriptor{
			PubKey: currentPubkey,
			KeyLocator: keychain.KeyLocator{
				Family: keychain.KeyFamilyMultiSig,
				Index:  index,
			},
		}, nil
	}

	return nil, fmt.Errorf("no matching pubkeys found")
}
