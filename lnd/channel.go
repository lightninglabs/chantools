package lnd

import (
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/input"
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
	theirSig := append(localCommit.CommitSig, byte(txscript.SigHashAll))

	// With this, we then generate the full witness so the caller can
	// broadcast a fully signed transaction.
	lc.SignDesc.SigHashes = txscript.NewTxSigHashes(commitTx)
	ourSigRaw, err := lc.TXSigner.SignOutputRaw(commitTx, lc.SignDesc)
	if err != nil {
		return nil, err
	}

	ourSig := append(ourSigRaw, byte(txscript.SigHashAll))

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
