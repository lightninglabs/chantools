package btc

import (
	"fmt"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/keychain"
)

type Signer struct {
	ExtendedKey *hdkeychain.ExtendedKey
	ChainParams *chaincfg.Params
}

func (s *Signer) SignOutputRaw(tx *wire.MsgTx,
	signDesc *input.SignDescriptor) ([]byte, error) {
	witnessScript := signDesc.WitnessScript

	// First attempt to fetch the private key which corresponds to the
	// specified public key.
	privKey, err := s.FetchPrivKey(&signDesc.KeyDesc)
	if err != nil {
		return nil, err
	}

	privKey = maybeTweakPrivKey(signDesc, privKey)
	amt := signDesc.Output.Value
	sig, err := txscript.RawTxInWitnessSignature(
		tx, signDesc.SigHashes, signDesc.InputIndex, amt,
		witnessScript, signDesc.HashType, privKey,
	)
	if err != nil {
		return nil, err
	}

	// Chop off the sighash flag at the end of the signature.
	return sig[:len(sig)-1], nil
}

func (s *Signer) ComputeInputScript(_ *wire.MsgTx, _ *input.SignDescriptor) (
	*input.Script, error) {

	return nil, fmt.Errorf("unimplemented")
}

func (s *Signer) FetchPrivKey(descriptor *keychain.KeyDescriptor) (
	*btcec.PrivateKey, error) {

	key, err := DeriveChildren(s.ExtendedKey, []uint32{
		HardenedKeyStart + uint32(keychain.BIP0043Purpose),
		HardenedKeyStart + s.ChainParams.HDCoinType,
		HardenedKeyStart + uint32(descriptor.Family),
		0,
		descriptor.Index,
	})
	if err != nil {
		return nil, err
	}
	return key.ECPrivKey()
}

// maybeTweakPrivKey examines the single tweak parameters on the passed sign
// descriptor and may perform a mapping on the passed private key in order to
// utilize the tweaks, if populated.
func maybeTweakPrivKey(signDesc *input.SignDescriptor,
	privKey *btcec.PrivateKey) *btcec.PrivateKey {

	if signDesc.SingleTweak != nil {
		return input.TweakPrivKey(privKey, signDesc.SingleTweak)
	}
	return privKey
}
