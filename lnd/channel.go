package lnd

import (
	"bytes"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcec/v2/ecdsa"
	"github.com/btcsuite/btcd/btcec/v2/schnorr/musig2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/lnwire"
)

type LightningChannel struct {
	LocalChanCfg  channeldb.ChannelConfig
	RemoteChanCfg channeldb.ChannelConfig
	SignDesc      *input.SignDescriptor
	ChannelState  *channeldb.OpenChannel
	TXSigner      *Signer
}

// CreateSignDesc derives the SignDescriptor for commitment transactions from
// other fields on the LightningChannel.
func (lc *LightningChannel) CreateSignDesc() error {
	localKey := lc.LocalChanCfg.MultiSigKey.PubKey.SerializeCompressed()
	remoteKey := lc.RemoteChanCfg.MultiSigKey.PubKey.SerializeCompressed()

	multiSigScript, err := input.GenMultiSigScript(localKey, remoteKey)
	if err != nil {
		return err
	}

	fundingPkScript, err := input.WitnessScriptHash(multiSigScript)
	if err != nil {
		return err
	}
	lc.SignDesc = &input.SignDescriptor{
		KeyDesc:       lc.LocalChanCfg.MultiSigKey,
		WitnessScript: multiSigScript,
		Output: &wire.TxOut{
			PkScript: fundingPkScript,
			Value:    int64(lc.ChannelState.Capacity),
		},
		HashType:   txscript.SigHashAll,
		InputIndex: 0,
		PrevOutputFetcher: txscript.NewCannedPrevOutputFetcher(
			fundingPkScript, int64(lc.ChannelState.Capacity),
		),
	}

	return nil
}

// SignedCommitTx function take the latest commitment transaction and populate
// it with witness data.
func (lc *LightningChannel) SignedCommitTx() (*wire.MsgTx, error) {
	// Fetch the current commitment transaction, along with their signature
	// for the transaction.
	localCommit := lc.ChannelState.LocalCommitment
	commitTx := localCommit.CommitTx.Copy()
	theirSig, err := ecdsa.ParseDERSignature(localCommit.CommitSig)
	if err != nil {
		return nil, err
	}

	// With this, we then generate the full witness so the caller can
	// broadcast a fully signed transaction.
	ourSig, err := lc.TXSigner.SignOutputRaw(commitTx, lc.SignDesc)
	if err != nil {
		return nil, err
	}

	// With the final signature generated, create the witness stack
	// required to spend from the multi-sig output.
	ourKey := lc.LocalChanCfg.MultiSigKey.PubKey.SerializeCompressed()
	theirKey := lc.RemoteChanCfg.MultiSigKey.PubKey.SerializeCompressed()

	commitTx.TxIn[0].Witness = input.SpendMultiSig(
		lc.SignDesc.WitnessScript, ourKey,
		ourSig, theirKey, theirSig,
	)

	return commitTx, nil
}

// ParseOutpoint parses a transaction outpoint in the format <txid>:<idx> into
// the wire format.
func ParseOutpoint(s string) (*wire.OutPoint, error) {
	split := strings.Split(s, ":")
	if len(split) != 2 {
		return nil, errors.New("expecting channel point to be in " +
			"format of: txid:index")
	}

	index, err := strconv.ParseInt(split[1], 10, 32)
	if err != nil {
		return nil, fmt.Errorf("unable to decode output index: %w",
			err)
	}

	txid, err := chainhash.NewHashFromStr(split[0])
	if err != nil {
		return nil, fmt.Errorf("unable to parse hex string: %w", err)
	}

	return &wire.OutPoint{
		Hash:  *txid,
		Index: uint32(index),
	}, nil
}

// GenerateMuSig2Nonces generates random nonces for a MuSig2 signing session.
func GenerateMuSig2Nonces(extendedKey *hdkeychain.ExtendedKey,
	randomness [32]byte, chanPoint *wire.OutPoint,
	chainParams *chaincfg.Params,
	signingKey *btcec.PrivateKey) (*musig2.Nonces, error) {

	privKey, err := DeriveMuSig2NoncePrivKey(extendedKey, chainParams)
	if err != nil {
		return nil, err
	}

	chanID := lnwire.NewChanIDFromOutPoint(*chanPoint)
	nonces, err := musig2.GenNonces(
		musig2.WithPublicKey(privKey.PubKey()),
		musig2.WithNonceSecretKeyAux(privKey),
		musig2.WithCustomRand(bytes.NewReader(randomness[:])),
		musig2.WithNonceAuxInput(chanID[:]),
	)
	if err != nil {
		return nil, err
	}

	// If we actually know the final signing key, we need to update it in
	// the secret nonce to bypass a check in the MuSig2 library.
	if signingKey != nil {
		copy(
			nonces.SecNonce[btcec.PrivKeyBytesLen*2:],
			signingKey.PubKey().SerializeCompressed(),
		)
	}

	return nonces, nil
}

// DeriveMuSig2NoncePrivKey derives a private key to be used as a nonce in a
// MuSig2 signing session.
func DeriveMuSig2NoncePrivKey(extendedKey *hdkeychain.ExtendedKey,
	chainParams *chaincfg.Params) (*btcec.PrivateKey, error) {

	// We use a derivation path that is not used by lnd, to make sure we
	// don't put any keys at risk.
	path := fmt.Sprintf(
		LndDerivationPath+"/0/%d", chainParams.HDCoinType, 1337, 1337,
	)

	key, _, _, err := DeriveKey(extendedKey, path, chainParams)
	if err != nil {
		return nil, err
	}

	return key.ECPrivKey()
}
