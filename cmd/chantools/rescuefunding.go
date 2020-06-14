package main

import (
	"bytes"
	"fmt"

	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/guggero/chantools/lnd"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/keychain"
)

const (
	MaxChannelLookup = 5000
)

type rescueFundingCommand struct {
	RootKey         string `long:"rootkey" description:"BIP32 HD root (m/) key to derive the key for our node from."`
	OtherNodePub    string `long:"othernodepub" description:"The extended public key (xpub) of the other node's multisig branch (m/1017'/<coin_type>'/0'/0)."`
	FundingAddr     string `long:"fundingaddr" description:"The bech32 script address of the funding output where the coins to be spent are locked in."`
	FundingOutpoint string `long:"fundingoutpoint" description:"The funding transaction outpoint (<txid>:<txindex>)."`
	FundingAmount   int64  `long:"fundingamount" description:"The exact amount in satoshis that is locked in the funding output."`
	SweepAddr       string `long:"sweepaddr" description:"The address to sweep the rescued funds to."`
	SatPerByte      int64  `long:"satperbyte" description:"The fee rate to use in satoshis/vByte."`
}

func (c *rescueFundingCommand) Execute(_ []string) error {
	setupChainParams(cfg)

	var (
		extendedKey *hdkeychain.ExtendedKey
		otherPub    *hdkeychain.ExtendedKey
		err         error
	)

	// Check that root key is valid or fall back to console input.
	switch {
	case c.RootKey != "":
		extendedKey, err = hdkeychain.NewKeyFromString(c.RootKey)

	default:
		extendedKey, _, err = lnd.ReadAezeedFromTerminal(chainParams)
	}
	if err != nil {
		return fmt.Errorf("error reading root key: %v", err)
	}

	// Read other node's xpub.
	otherPub, err = hdkeychain.NewKeyFromString(c.OtherNodePub)
	if err != nil {
		return fmt.Errorf("error parsing other node's xpub: %v", err)
	}

	// Decode target funding address.
	hash, isScript, err := lnd.DecodeAddressHash(c.FundingAddr, chainParams)
	if err != nil {
		return fmt.Errorf("error decoding funding address: %v", err)
	}
	if !isScript {
		return fmt.Errorf("funding address must be a P2WSH address")
	}

	return rescueFunding(extendedKey, otherPub, hash)
}

func rescueFunding(localNodeKey *hdkeychain.ExtendedKey,
	otherNodekey *hdkeychain.ExtendedKey, scriptHash []byte) error {

	// First, we need to derive the correct branch from the local root key.
	localMultisig, err := lnd.DeriveChildren(localNodeKey, []uint32{
		lnd.HardenedKeyStart + uint32(keychain.BIP0043Purpose),
		lnd.HardenedKeyStart + chainParams.HDCoinType,
		lnd.HardenedKeyStart + uint32(keychain.KeyFamilyMultiSig),
		0,
	})
	if err != nil {
		return fmt.Errorf("could not derive local multisig key: %v",
			err)
	}

	log.Infof("Looking for matching multisig keys, this will take a while")
	localIndex, otherIndex, script, err := findMatchingIndices(
		localMultisig, otherNodekey, scriptHash,
	)
	if err != nil {
		return fmt.Errorf("could not derive keys: %v", err)
	}

	log.Infof("Found local key with index %d and other key with index %d "+
		"for witness script %x", localIndex, otherIndex, script)

	// TODO(guggero):
	//  * craft PSBT with input, sweep output and partial signature
	//  * do fee estimation based on full amount
	//  * create `signpsbt` command for the other node operator
	return nil
}

func findMatchingIndices(localNodeKey *hdkeychain.ExtendedKey,
	otherNodekey *hdkeychain.ExtendedKey, scriptHash []byte) (uint32,
	uint32, []byte, error) {

	// Loop through both the local and the remote indices of the branches up
	// to MaxChannelLookup.
	for local := uint32(0); local < MaxChannelLookup; local++ {
		for other := uint32(0); other < MaxChannelLookup; other++ {
			localKey, err := localNodeKey.Child(local)
			if err != nil {
				return 0, 0, nil, fmt.Errorf("error "+
					"deriving local key: %v", err)
			}
			localPub, err := localKey.ECPubKey()
			if err != nil {
				return 0, 0, nil, fmt.Errorf("error "+
					"deriving local pubkey: %v", err)
			}
			otherKey, err := otherNodekey.Child(other)
			if err != nil {
				return 0, 0, nil, fmt.Errorf("error "+
					"deriving other key: %v", err)
			}
			otherPub, err := otherKey.ECPubKey()
			if err != nil {
				return 0, 0, nil, fmt.Errorf("error "+
					"deriving other pubkey: %v", err)
			}
			script, out, err := input.GenFundingPkScript(
				localPub.SerializeCompressed(),
				otherPub.SerializeCompressed(), 123,
			)
			if err != nil {
				return 0, 0, nil, fmt.Errorf("error "+
					"generating funding script: %v", err)
			}
			if bytes.Contains(out.PkScript, scriptHash) {
				return local, other, script, nil
			}
		}
		if local > 0 && local%100 == 0 {
			log.Infof("Checked %d of %d local keys", local,
				MaxChannelLookup)
		}
	}
	return 0, 0, nil, fmt.Errorf("no matching pubkeys found")
}
