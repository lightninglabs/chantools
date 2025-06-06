package cln

import (
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/psbt"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btcwallet/wallet"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/keychain"
)

type Signer struct {
	*input.MusigSessionManager

	HsmSecret [32]byte

	// SwapDescKeyAfterDerive is a boolean that indicates that after
	// deriving the private key from the key descriptor (which interprets
	// the public key as the peer's public key), we should swap the public
	// key in the key descriptor to the actual derived public key. This is
	// required for P2WKH signatures that need to have the public key in the
	// witness stack.
	SwapDescKeyAfterDerive bool
}

func (s *Signer) SignOutputRaw(tx *wire.MsgTx,
	signDesc *input.SignDescriptor) (input.Signature, error) {

	// First attempt to fetch the private key which corresponds to the
	// specified public key.
	privKey, err := s.FetchPrivateKey(&signDesc.KeyDesc)
	if err != nil {
		return nil, err
	}

	if s.SwapDescKeyAfterDerive {
		// If we need to swap the public key in the descriptor, we do so
		// now. This is required for P2WKH signatures that need to have
		// the public key in the witness stack.
		signDesc.KeyDesc.PubKey = privKey.PubKey()
	}

	return lnd.SignOutputRawWithPrivateKey(tx, signDesc, privKey)
}

func (s *Signer) ComputeInputScript(_ *wire.MsgTx, _ *input.SignDescriptor) (
	*input.Script, error) {

	return nil, errors.New("unimplemented")
}

func (s *Signer) FetchPrivateKey(
	descriptor *keychain.KeyDescriptor) (*btcec.PrivateKey, error) {

	_, privKey, err := DeriveKeyPair(s.HsmSecret, descriptor)
	return privKey, err
}

func (s *Signer) FindMultisigKey(targetPubkey, peerPubKey *btcec.PublicKey,
	maxNumKeys uint32) (*keychain.KeyDescriptor, error) {

	// Loop through the local multisig keys to find the target key.
	for index := range maxNumKeys {
		privKey, err := s.FetchPrivateKey(&keychain.KeyDescriptor{
			PubKey: peerPubKey,
			KeyLocator: keychain.KeyLocator{
				Family: keychain.KeyFamilyMultiSig,
				Index:  index,
			},
		})
		if err != nil {
			return nil, fmt.Errorf("error deriving funding "+
				"private key: %w", err)
		}

		currentPubkey := privKey.PubKey()
		if !targetPubkey.IsEqual(currentPubkey) {
			continue
		}

		return &keychain.KeyDescriptor{
			PubKey: peerPubKey,
			KeyLocator: keychain.KeyLocator{
				Family: keychain.KeyFamilyMultiSig,
				Index:  index,
			},
		}, nil
	}

	return nil, errors.New("no matching pubkeys found")
}

func (s *Signer) AddPartialSignature(packet *psbt.Packet,
	keyDesc keychain.KeyDescriptor, utxo *wire.TxOut, witnessScript []byte,
	inputIndex int) error {

	// Now we add our partial signature.
	prevOutFetcher := wallet.PsbtPrevOutputFetcher(packet)
	signDesc := &input.SignDescriptor{
		KeyDesc:           keyDesc,
		WitnessScript:     witnessScript,
		Output:            utxo,
		InputIndex:        inputIndex,
		HashType:          txscript.SigHashAll,
		PrevOutputFetcher: prevOutFetcher,
		SigHashes: txscript.NewTxSigHashes(
			packet.UnsignedTx, prevOutFetcher,
		),
	}
	ourSigRaw, err := s.SignOutputRaw(packet.UnsignedTx, signDesc)
	if err != nil {
		return fmt.Errorf("error signing with our key: %w", err)
	}
	ourSig := append(ourSigRaw.Serialize(), byte(txscript.SigHashAll))

	// Because of the way we derive keys in CLN, the public key in the key
	// descriptor is the peer's public key, not our own. So we need to
	// derive our own public key from the private key.
	ourPrivKey, err := s.FetchPrivateKey(&keyDesc)
	if err != nil {
		return fmt.Errorf("error fetching private key for descriptor "+
			"%v: %w", keyDesc, err)
	}
	ourPubKey := ourPrivKey.PubKey()

	// Great, we were able to create our sig, let's add it to the PSBT.
	updater, err := psbt.NewUpdater(packet)
	if err != nil {
		return fmt.Errorf("error creating PSBT updater: %w", err)
	}
	status, err := updater.Sign(
		inputIndex, ourSig, ourPubKey.SerializeCompressed(), nil,
		witnessScript,
	)
	if err != nil {
		return fmt.Errorf("error adding signature to PSBT: %w", err)
	}
	if status != 0 {
		return fmt.Errorf("unexpected status for signature update, "+
			"got %d wanted 0", status)
	}

	return nil
}

var _ lnd.ChannelSigner = (*Signer)(nil)
