package main

import (
	"bytes"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/psbt"
	"github.com/lightninglabs/chantools/lnd"
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

	return signRescueFunding(packet, signer)
}

func signRescueFunding(packet *psbt.Packet, signer *lnd.Signer) error {
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
	// add our signature. This is NOT CLN compatible, as we'd need to
	// add the peer's public key as a command argument to pass into
	// FindMultisigKey.
	localKeyDesc, err := signer.FindMultisigKey(
		targetKey, nil, MaxChannelLookup,
	)
	if err != nil {
		return fmt.Errorf("could not find local multisig key: %w", err)
	}
	if len(packet.Inputs[0].WitnessScript) == 0 {
		return errors.New("invalid PSBT, missing witness script")
	}
	witnessScript := packet.Inputs[0].WitnessScript
	if packet.Inputs[0].WitnessUtxo == nil {
		return errors.New("invalid PSBT, witness UTXO missing")
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
