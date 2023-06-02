package main

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"time"

	"github.com/lightninglabs/chantools/lnd"
	"github.com/spf13/cobra"
)

const (
	numMultisigKeys = 2500
)

type zombieRecoveryPrepareKeysCommand struct {
	MatchFile  string
	PayoutAddr string

	rootKey *rootKey
	cmd     *cobra.Command
}

func newZombieRecoveryPrepareKeysCommand() *cobra.Command {
	cc := &zombieRecoveryPrepareKeysCommand{}
	cc.cmd = &cobra.Command{
		Use:   "preparekeys",
		Short: "[1/3] Prepare all public keys for a recovery attempt",
		Long: `Takes a match file, validates it against the seed and 
then adds the first 2500 multisig pubkeys to it.
This must be run by both parties of a channel for a successful recovery. The
next step (makeoffer) takes two such key enriched files and tries to find the
correct ones for the matched channels.`,
		Example: `chantools zombierecovery preparekeys \
	--match_file match-xxxx-xx-xx-<pubkey1>-<pubkey2>.json \
	--payout_addr bc1q...`,
		RunE: cc.Execute,
	}

	cc.cmd.Flags().StringVar(
		&cc.MatchFile, "match_file", "", "the match JSON file that "+
			"was sent to both nodes by the match maker",
	)
	cc.cmd.Flags().StringVar(
		&cc.PayoutAddr, "payout_addr", "", "the address where this "+
			"node's rescued funds should be sent to, must be a "+
			"P2WPKH (native SegWit) address")

	cc.rootKey = newRootKey(cc.cmd, "deriving the multisig keys")

	return cc.cmd
}

func (c *zombieRecoveryPrepareKeysCommand) Execute(_ *cobra.Command,
	_ []string) error {

	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %w", err)
	}

	_, err = lnd.GetP2WPKHScript(c.PayoutAddr, chainParams)
	if err != nil {
		return fmt.Errorf("invalid payout address, must be P2WPKH")
	}

	matchFileBytes, err := ioutil.ReadFile(c.MatchFile)
	if err != nil {
		return fmt.Errorf("error reading match file %s: %w",
			c.MatchFile, err)
	}

	decoder := json.NewDecoder(bytes.NewReader(matchFileBytes))
	match := &match{}
	if err := decoder.Decode(&match); err != nil {
		return fmt.Errorf("error decoding match file %s: %w",
			c.MatchFile, err)
	}

	// Make sure the match file was filled correctly.
	if match.Node1 == nil || match.Node2 == nil {
		return fmt.Errorf("invalid match file, node info missing")
	}

	_, pubKey, _, err := lnd.DeriveKey(
		extendedKey, lnd.IdentityPath(chainParams), chainParams,
	)
	if err != nil {
		return fmt.Errorf("error deriving identity pubkey: %w", err)
	}

	pubKeyStr := hex.EncodeToString(pubKey.SerializeCompressed())
	var nodeInfo *nodeInfo
	switch {
	case match.Node1.PubKey != pubKeyStr && match.Node2.PubKey != pubKeyStr:
		return fmt.Errorf("derived pubkey %s from seed but that key "+
			"was not found in the match file %s", pubKeyStr,
			c.MatchFile)

	case match.Node1.PubKey == pubKeyStr:
		nodeInfo = match.Node1

	default:
		nodeInfo = match.Node2
	}

	// Derive all 2500 keys now, this might take a while.
	for index := 0; index < numMultisigKeys; index++ {
		_, pubKey, _, err := lnd.DeriveKey(
			extendedKey, lnd.MultisigPath(chainParams, index),
			chainParams,
		)
		if err != nil {
			return fmt.Errorf("error deriving multisig pubkey: %w",
				err)
		}

		nodeInfo.MultisigKeys = append(
			nodeInfo.MultisigKeys,
			hex.EncodeToString(pubKey.SerializeCompressed()),
		)
	}
	nodeInfo.PayoutAddr = c.PayoutAddr

	// Write the result back into a new file.
	matchBytes, err := json.MarshalIndent(match, "", " ")
	if err != nil {
		return err
	}

	fileName := fmt.Sprintf("results/preparedkeys-%s-%s.json",
		time.Now().Format("2006-01-02"), pubKeyStr)
	log.Infof("Writing result to %s", fileName)
	return ioutil.WriteFile(fileName, matchBytes, 0644)
}
