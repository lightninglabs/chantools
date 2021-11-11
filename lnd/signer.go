package lnd

import (
	"crypto/sha256"
	"fmt"

	"github.com/btcsuite/btcd/btcec"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/btcsuite/btcutil/psbt"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/keychain"
)

type Signer struct {
	ExtendedKey *hdkeychain.ExtendedKey
	ChainParams *chaincfg.Params
}

func (s *Signer) SignOutputRaw(tx *wire.MsgTx,
	signDesc *input.SignDescriptor) (input.Signature, error) {
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
	return btcec.ParseDERSignature(sig[:len(sig)-1], btcec.S256())
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

func (s *Signer) AddPartialSignature(packet *psbt.Packet,
	keyDesc keychain.KeyDescriptor, utxo *wire.TxOut, witnessScript []byte,
	inputIndex int) error {

	// Now we add our partial signature.
	signDesc := &input.SignDescriptor{
		KeyDesc:       keyDesc,
		WitnessScript: witnessScript,
		Output:        utxo,
		InputIndex:    inputIndex,
		HashType:      txscript.SigHashAll,
		SigHashes:     txscript.NewTxSigHashes(packet.UnsignedTx),
	}
	ourSigRaw, err := s.SignOutputRaw(packet.UnsignedTx, signDesc)
	if err != nil {
		return fmt.Errorf("error signing with our key: %v", err)
	}
	ourSig := append(ourSigRaw.Serialize(), byte(txscript.SigHashAll))

	// Great, we were able to create our sig, let's add it to the PSBT.
	updater, err := psbt.NewUpdater(packet)
	if err != nil {
		return fmt.Errorf("error creating PSBT updater: %v", err)
	}
	status, err := updater.Sign(
		inputIndex, ourSig, keyDesc.PubKey.SerializeCompressed(), nil,
		witnessScript,
	)
	if err != nil {
		return fmt.Errorf("error adding signature to PSBT: %v", err)
	}
	if status != 0 {
		return fmt.Errorf("unexpected status for signature update, "+
			"got %d wanted 0", status)
	}

	return nil
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

// ECDH performs a scalar multiplication (ECDH-like operation) between the
// target private key and remote public key. The output returned will be
// the sha256 of the resulting shared point serialized in compressed format. If
// k is our private key, and P is the public key, we perform the following
// operation:
//
//  sx := k*P s := sha256(sx.SerializeCompressed())
func ECDH(privKey *btcec.PrivateKey, pub *btcec.PublicKey) ([32]byte, error) {
	s := &btcec.PublicKey{}
	x, y := btcec.S256().ScalarMult(pub.X, pub.Y, privKey.D.Bytes())
	s.X = x
	s.Y = y

	h := sha256.Sum256(s.SerializeCompressed())

	return h, nil
}
