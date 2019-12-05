package chantools

import (
	"fmt"
	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/keychain"
)

type signer struct {
	extendedKey *hdkeychain.ExtendedKey
}

func (s *signer) SignOutputRaw(tx *wire.MsgTx,
	signDesc *input.SignDescriptor) ([]byte, error) {
	witnessScript := signDesc.WitnessScript

	// First attempt to fetch the private key which corresponds to the
	// specified public key.
	privKey, err := s.fetchPrivKey(&signDesc.KeyDesc)
	if err != nil {
		return nil, err
	}
	
	privKey, err = maybeTweakPrivKey(signDesc, privKey)
	if err != nil {
		return nil, err
	}

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

func (s *signer) ComputeInputScript(tx *wire.MsgTx,
	signDesc *input.SignDescriptor) (*input.Script, error) {
	return nil, fmt.Errorf("unimplemented")
}

func (s *signer) fetchPrivKey(descriptor *keychain.KeyDescriptor) (
	*btcec.PrivateKey, error) {

	key, err := deriveChildren(s.extendedKey, []uint32{
		hardenedKeyStart + uint32(keychain.BIP0043Purpose),
		hardenedKeyStart + chainParams.HDCoinType,
		hardenedKeyStart + uint32(descriptor.Family),
		0,
		descriptor.Index,
	})
	if err != nil {
		return nil, err
	}
	return key.ECPrivKey()
}

// maybeTweakPrivKey examines the single and double tweak parameters on the
// passed sign descriptor and may perform a mapping on the passed private key
// in order to utilize the tweaks, if populated.
func maybeTweakPrivKey(signDesc *input.SignDescriptor,
	privKey *btcec.PrivateKey) (*btcec.PrivateKey, error) {

	var retPriv *btcec.PrivateKey
	switch {

	case signDesc.SingleTweak != nil:
		retPriv = input.TweakPrivKey(privKey,
			signDesc.SingleTweak)

	case signDesc.DoubleTweak != nil:
		retPriv = input.DeriveRevocationPrivKey(privKey,
			signDesc.DoubleTweak)

	default:
		retPriv = privKey
	}

	return retPriv, nil
}