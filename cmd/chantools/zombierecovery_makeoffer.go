package main

import (
	"bufio"
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"os"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/psbt"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcwallet/wallet/txrules"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lnwallet/chainfee"
	"github.com/spf13/cobra"
)

type zombieRecoveryMakeOfferCommand struct {
	Node1   string
	Node2   string
	FeeRate uint32

	rootKey *rootKey
	cmd     *cobra.Command
}

func newZombieRecoveryMakeOfferCommand() *cobra.Command {
	cc := &zombieRecoveryMakeOfferCommand{}
	cc.cmd = &cobra.Command{
		Use: "makeoffer",
		Short: "[2/3] Make an offer on how to split the funds to " +
			"recover",
		Long: `After both parties have prepared their keys with the
'preparekeys' command and have  exchanged the files generated from that step,
one party has to create an offer on how to split the funds that are in the
channels to be rescued.
If the other party agrees with the offer, they can sign and publish the offer
with the 'signoffer' command. If the other party does not agree, they can create
a counter offer.`,
		Example: `chantools zombierecovery makeoffer \
	--node1_keys preparedkeys-xxxx-xx-xx-<pubkey1>.json \
	--node2_keys preparedkeys-xxxx-xx-xx-<pubkey2>.json \
	--feerate 15`,
		RunE: cc.Execute,
	}

	cc.cmd.Flags().StringVar(
		&cc.Node1, "node1_keys", "", "the JSON file generated in the"+
			"previous step ('preparekeys') command of node 1",
	)
	cc.cmd.Flags().StringVar(
		&cc.Node2, "node2_keys", "", "the JSON file generated in the"+
			"previous step ('preparekeys') command of node 2",
	)
	cc.cmd.Flags().Uint32Var(
		&cc.FeeRate, "feerate", defaultFeeSatPerVByte, "fee rate to "+
			"use for the sweep transaction in sat/vByte",
	)

	cc.rootKey = newRootKey(cc.cmd, "signing the offer")

	return cc.cmd
}

func (c *zombieRecoveryMakeOfferCommand) Execute(_ *cobra.Command,
	_ []string) error {

	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %w", err)
	}

	if c.FeeRate == 0 {
		c.FeeRate = defaultFeeSatPerVByte
	}

	node1Bytes, err := ioutil.ReadFile(c.Node1)
	if err != nil {
		return fmt.Errorf("error reading node1 key file %s: %w",
			c.Node1, err)
	}
	node2Bytes, err := ioutil.ReadFile(c.Node2)
	if err != nil {
		return fmt.Errorf("error reading node2 key file %s: %w",
			c.Node2, err)
	}
	keys1, keys2 := &match{}, &match{}
	decoder := json.NewDecoder(bytes.NewReader(node1Bytes))
	if err := decoder.Decode(&keys1); err != nil {
		return fmt.Errorf("error decoding node1 key file %s: %w",
			c.Node1, err)
	}
	decoder = json.NewDecoder(bytes.NewReader(node2Bytes))
	if err := decoder.Decode(&keys2); err != nil {
		return fmt.Errorf("error decoding node2 key file %s: %w",
			c.Node2, err)
	}

	// Make sure the key files were filled correctly.
	if keys1.Node1 == nil || keys1.Node2 == nil {
		return fmt.Errorf("invalid node1 file, node info missing")
	}
	if keys2.Node1 == nil || keys2.Node2 == nil {
		return fmt.Errorf("invalid node2 file, node info missing")
	}
	if keys1.Node1.PubKey != keys2.Node1.PubKey {
		return fmt.Errorf("invalid files, node 1 pubkey doesn't match")
	}
	if keys1.Node2.PubKey != keys2.Node2.PubKey {
		return fmt.Errorf("invalid files, node 2 pubkey doesn't match")
	}
	if len(keys1.Node1.MultisigKeys) == 0 &&
		len(keys1.Node2.MultisigKeys) == 0 {

		return fmt.Errorf("invalid node1 file, missing multisig keys")
	}
	if len(keys2.Node1.MultisigKeys) == 0 &&
		len(keys2.Node2.MultisigKeys) == 0 {

		return fmt.Errorf("invalid node2 file, missing multisig keys")
	}
	if len(keys1.Node1.MultisigKeys) == len(keys2.Node1.MultisigKeys) {
		return fmt.Errorf("invalid files, channel info incorrect")
	}
	if len(keys1.Node2.MultisigKeys) == len(keys2.Node2.MultisigKeys) {
		return fmt.Errorf("invalid files, channel info incorrect")
	}
	if len(keys1.Channels) != len(keys2.Channels) {
		return fmt.Errorf("invalid files, channels don't match")
	}
	for idx, node1Channel := range keys1.Channels {
		if keys2.Channels[idx].ChanPoint != node1Channel.ChanPoint {
			return fmt.Errorf("invalid files, channels don't match")
		}

		if keys2.Channels[idx].Address != node1Channel.Address {
			return fmt.Errorf("invalid files, channels don't match")
		}

		if keys2.Channels[idx].Address == "" ||
			node1Channel.Address == "" {

			return fmt.Errorf("invalid files, channel address " +
				"missing")
		}
	}

	// Make sure one of the nodes is ours.
	_, pubKey, _, err := lnd.DeriveKey(
		extendedKey, lnd.IdentityPath(chainParams), chainParams,
	)
	if err != nil {
		return fmt.Errorf("error deriving identity pubkey: %w", err)
	}

	pubKeyStr := hex.EncodeToString(pubKey.SerializeCompressed())
	if keys1.Node1.PubKey != pubKeyStr && keys1.Node2.PubKey != pubKeyStr {
		return fmt.Errorf("derived pubkey %s from seed but that key "+
			"was not found in the match files", pubKeyStr)
	}

	// Pick the correct list of keys. There are 4 possibilities, given 2
	// files with 2 node slots each.
	var (
		ourKeys         []string
		ourPayoutAddr   string
		theirKeys       []string
		theirPayoutAddr string
	)
	if keys1.Node1.PubKey == pubKeyStr && len(keys1.Node1.MultisigKeys) > 0 {
		ourKeys = keys1.Node1.MultisigKeys
		ourPayoutAddr = keys1.Node1.PayoutAddr
		theirKeys = keys2.Node2.MultisigKeys
		theirPayoutAddr = keys2.Node2.PayoutAddr
	}
	if keys1.Node2.PubKey == pubKeyStr && len(keys1.Node2.MultisigKeys) > 0 {
		ourKeys = keys1.Node2.MultisigKeys
		ourPayoutAddr = keys1.Node2.PayoutAddr
		theirKeys = keys2.Node1.MultisigKeys
		theirPayoutAddr = keys2.Node1.PayoutAddr
	}
	if keys2.Node1.PubKey == pubKeyStr && len(keys2.Node1.MultisigKeys) > 0 {
		ourKeys = keys2.Node1.MultisigKeys
		ourPayoutAddr = keys2.Node1.PayoutAddr
		theirKeys = keys1.Node2.MultisigKeys
		theirPayoutAddr = keys1.Node2.PayoutAddr
	}
	if keys2.Node2.PubKey == pubKeyStr && len(keys2.Node2.MultisigKeys) > 0 {
		ourKeys = keys2.Node2.MultisigKeys
		ourPayoutAddr = keys2.Node2.PayoutAddr
		theirKeys = keys1.Node1.MultisigKeys
		theirPayoutAddr = keys1.Node1.PayoutAddr
	}
	if len(ourKeys) == 0 || len(theirKeys) == 0 {
		return fmt.Errorf("couldn't find necessary keys")
	}
	if ourPayoutAddr == "" || theirPayoutAddr == "" {
		return fmt.Errorf("payout address missing")
	}

	ourPubKeys := make([]*btcec.PublicKey, len(ourKeys))
	theirPubKeys := make([]*btcec.PublicKey, len(theirKeys))
	for idx, pubKeyHex := range ourKeys {
		ourPubKeys[idx], err = pubKeyFromHex(pubKeyHex)
		if err != nil {
			return fmt.Errorf("error parsing our pubKey: %w", err)
		}
	}
	for idx, pubKeyHex := range theirKeys {
		theirPubKeys[idx], err = pubKeyFromHex(pubKeyHex)
		if err != nil {
			return fmt.Errorf("error parsing their pubKey: %w", err)
		}
	}

	// Loop through all channels and all keys now, this will definitely take
	// a while.
channelLoop:
	for _, channel := range keys1.Channels {
		for ourKeyIndex, ourKey := range ourPubKeys {
			for _, theirKey := range theirPubKeys {
				match, witnessScript, err := matchScript(
					channel.Address, ourKey, theirKey,
					chainParams,
				)
				if err != nil {
					return fmt.Errorf("error matching "+
						"keys to script: %w", err)
				}

				if match {
					channel.ourKeyIndex = uint32(ourKeyIndex)
					channel.ourKey = ourKey
					channel.theirKey = theirKey
					channel.witnessScript = witnessScript

					log.Infof("Found keys for channel %s",
						channel.ChanPoint)

					continue channelLoop
				}
			}
		}

		return fmt.Errorf("didn't find matching multisig keys for "+
			"channel %s", channel.ChanPoint)
	}

	// Let's now sum up the tally of how much of the rescued funds should
	// go to which party.
	var (
		inputs   = make([]*wire.TxIn, 0, len(keys1.Channels))
		ourSum   int64
		theirSum int64
	)
	for idx, channel := range keys1.Channels {
		op, err := lnd.ParseOutpoint(channel.ChanPoint)
		if err != nil {
			return fmt.Errorf("error parsing channel out point: %w",
				err)
		}
		channel.txid = op.Hash.String()
		channel.vout = op.Index

		ourPart, theirPart, err := askAboutChannel(
			channel, idx+1, len(keys1.Channels), ourPayoutAddr,
			theirPayoutAddr,
		)
		if err != nil {
			return err
		}

		ourSum += ourPart
		theirSum += theirPart
		inputs = append(inputs, &wire.TxIn{
			PreviousOutPoint: *op,
			// It's not actually an old sig script but a witness
			// script but we'll move that to the correct place once
			// we create the PSBT.
			SignatureScript: channel.witnessScript,
		})
	}

	// Let's create a fee estimator now to give an overview over the
	// deducted fees.
	estimator := input.TxWeightEstimator{}

	// Only add output for us if we should receive something.
	if ourSum > 0 {
		estimator.AddP2WKHOutput()
	}
	if theirSum > 0 {
		estimator.AddP2WKHOutput()
	}
	for range inputs {
		estimator.AddWitnessInput(input.MultiSigWitnessSize)
	}
	feeRateKWeight := chainfee.SatPerKVByte(1000 * c.FeeRate).FeePerKWeight()
	totalFee := int64(feeRateKWeight.FeeForWeight(int64(estimator.Weight())))

	fmt.Printf("Current tally (before fees):\n\t"+
		"To our address (%s): %d sats\n\t"+
		"To their address (%s): %d sats\n\t"+
		"Estimated fees (at rate %d sat/vByte): %d sats\n",
		ourPayoutAddr, ourSum, theirPayoutAddr, theirSum, c.FeeRate,
		totalFee)

	// Distribute the fees.
	halfFee := totalFee / 2
	switch {
	case ourSum-halfFee > 0 && theirSum-halfFee > 0:
		ourSum -= halfFee
		theirSum -= halfFee

	case ourSum-totalFee > 0:
		ourSum -= totalFee

	case theirSum-totalFee > 0:
		theirSum -= totalFee

	default:
		return fmt.Errorf("error distributing fees, unhandled case")
	}

	// Our output.
	pkScript, err := lnd.GetP2WPKHScript(ourPayoutAddr, chainParams)
	if err != nil {
		return fmt.Errorf("error parsing our payout address: %w", err)
	}
	ourTxOut := &wire.TxOut{
		PkScript: pkScript,
		Value:    ourSum,
	}

	// Their output
	pkScript, err = lnd.GetP2WPKHScript(theirPayoutAddr, chainParams)
	if err != nil {
		return fmt.Errorf("error parsing their payout address: %w", err)
	}
	theirTxOut := &wire.TxOut{
		PkScript: pkScript,
		Value:    theirSum,
	}

	// Don't create dust.
	if txrules.IsDustOutput(ourTxOut, txrules.DefaultRelayFeePerKb) {
		ourSum = 0
	}
	if txrules.IsDustOutput(theirTxOut, txrules.DefaultRelayFeePerKb) {
		theirSum = 0
	}

	fmt.Printf("Current tally (after fees):\n\t"+
		"To our address (%s): %d sats\n\t"+
		"To their address (%s): %d sats\n",
		ourPayoutAddr, ourSum, theirPayoutAddr, theirSum)

	// And now create the PSBT.
	tx := wire.NewMsgTx(2)
	if ourSum > 0 {
		tx.TxOut = append(tx.TxOut, ourTxOut)
	}
	if theirSum > 0 {
		tx.TxOut = append(tx.TxOut, theirTxOut)
	}
	for _, txIn := range inputs {
		tx.TxIn = append(tx.TxIn, &wire.TxIn{
			PreviousOutPoint: txIn.PreviousOutPoint,
		})
	}
	packet, err := psbt.NewFromUnsignedTx(tx)
	if err != nil {
		return fmt.Errorf("error creating PSBT from TX: %w", err)
	}

	signer := &lnd.Signer{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}
	for idx, txIn := range inputs {
		channel := keys1.Channels[idx]

		// We've mis-used this field to transport the witness script,
		// let's now copy it to the correct place.
		packet.Inputs[idx].WitnessScript = txIn.SignatureScript

		// Let's prepare the witness UTXO.
		pkScript, err := input.WitnessScriptHash(channel.witnessScript)
		if err != nil {
			return err
		}
		packet.Inputs[idx].WitnessUtxo = &wire.TxOut{
			PkScript: pkScript,
			Value:    channel.Capacity,
		}

		// We'll be signing with our key so we can just add the other
		// party's pubkey as additional info so it's easy for them to
		// sign as well.
		packet.Inputs[idx].Unknowns = append(
			packet.Inputs[idx].Unknowns, &psbt.Unknown{
				Key:   PsbtKeyTypeOutputMissingSigPubkey,
				Value: channel.theirKey.SerializeCompressed(),
			},
		)

		keyDesc := keychain.KeyDescriptor{
			PubKey: channel.ourKey,
			KeyLocator: keychain.KeyLocator{
				Family: keychain.KeyFamilyMultiSig,
				Index:  channel.ourKeyIndex,
			},
		}
		utxo := &wire.TxOut{
			Value: channel.Capacity,
		}
		err = signer.AddPartialSignature(
			packet, keyDesc, utxo, txIn.SignatureScript, idx,
		)
		if err != nil {
			return fmt.Errorf("error signing input %d: %w", idx,
				err)
		}
	}

	// Looks like we're done!
	base64, err := packet.B64Encode()
	if err != nil {
		return fmt.Errorf("error encoding PSBT: %w", err)
	}

	fmt.Printf("Done creating offer, please send this PSBT string to \n"+
		"the other party to review and sign (if they accept): \n%s\n",
		base64)

	return nil
}

func matchScript(address string, key1, key2 *btcec.PublicKey,
	params *chaincfg.Params) (bool, []byte, error) {

	channelScript, err := lnd.GetP2WSHScript(address, params)
	if err != nil {
		return false, nil, err
	}

	witnessScript, err := input.GenMultiSigScript(
		key1.SerializeCompressed(), key2.SerializeCompressed(),
	)
	if err != nil {
		return false, nil, err
	}
	pkScript, err := input.WitnessScriptHash(witnessScript)
	if err != nil {
		return false, nil, err
	}

	return bytes.Equal(channelScript, pkScript), witnessScript, nil
}

func askAboutChannel(channel *channel, current, total int, ourAddr,
	theirAddr string) (int64, int64, error) {

	fundingTxid := strings.Split(channel.ChanPoint, ":")[0]

	fmt.Printf("Channel %s (%d of %d): \n\tCapacity: %d sat\n\t"+
		"Funding TXID: https://blockstream.info/tx/%v\n\t"+
		"Channel info: https://1ml.com/channel/%s\n\t"+
		"Channel funding address: %s\n\n"+
		"How many sats should go to you (%s) before fees?: ",
		channel.ChanPoint, current, total, channel.Capacity,
		fundingTxid, channel.ChannelID, channel.Address, ourAddr)
	reader := bufio.NewReader(os.Stdin)
	ourPartStr, err := reader.ReadString('\n')
	if err != nil {
		return 0, 0, err
	}

	ourPart, err := strconv.ParseUint(strings.TrimSpace(ourPartStr), 10, 64)
	if err != nil {
		return 0, 0, err
	}

	// Let the user try again if they entered something incorrect.
	if int64(ourPart) > channel.Capacity {
		fmt.Printf("Cannot send more than %d sats to ourself!\n",
			channel.Capacity)
		return askAboutChannel(
			channel, current, total, ourAddr, theirAddr,
		)
	}

	theirPart := channel.Capacity - int64(ourPart)
	fmt.Printf("\nWill send: \n\t%d sats to our address (%s) and \n\t"+
		"%d sats to the other peer's address (%s).\n\n", ourPart,
		ourAddr, theirPart, theirAddr)

	return int64(ourPart), theirPart, nil
}
