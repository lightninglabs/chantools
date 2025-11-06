package main

import (
	"bytes"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/wire"
	"github.com/btcsuite/btclog/v2"
	"github.com/lightningnetwork/lnd/chanbackup"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/stretchr/testify/require"
)

// TestClassifyOutputs_RealData verifies we can identify the to_remote output
// using lnwallet.CommitScriptToRemote with real world data provided.
func TestClassifyOutputs_RealData(t *testing.T) {
	h := newHarness(t)
	h.logger.SetLevel(btclog.LevelTrace)

	// Load test data from embedded file.
	testDataBytes := h.readTestdataFile("scbforceclose_testdata.json")

	var testData struct {
		RemotePubkey   string `json:"remote_pubkey"`
		TransactionHex string `json:"transaction_hex"`
	}
	err := json.Unmarshal(testDataBytes, &testData)
	require.NoError(t, err)

	// Remote payment basepoint (compressed) provided by user.
	remoteBytes, err := hex.DecodeString(testData.RemotePubkey)
	require.NoError(t, err)
	remoteKey, err := btcec.ParsePubKey(remoteBytes)
	require.NoError(t, err)

	// Example transaction hex from a real world channel.
	txBytes, err := hex.DecodeString(testData.TransactionHex)
	require.NoError(t, err)
	var tx wire.MsgTx
	require.NoError(t, tx.Deserialize(bytes.NewReader(txBytes)))

	// Build a minimal Single with the remote payment basepoint.
	makeSingle := func(version chanbackup.SingleBackupVersion,
		initiator bool) chanbackup.Single {

		s := chanbackup.Single{
			Version:     version,
			IsInitiator: initiator,
		}
		s.RemoteChanCfg.PaymentBasePoint = keychain.KeyDescriptor{
			PubKey: remoteKey,
		}

		return s
	}

	// Try a set of plausible SCB versions and initiator roles to find
	// a match.
	versions := []chanbackup.SingleBackupVersion{
		chanbackup.AnchorsCommitVersion,
		chanbackup.AnchorsZeroFeeHtlcTxCommitVersion,
		chanbackup.ScriptEnforcedLeaseVersion,
		chanbackup.TweaklessCommitVersion,
		chanbackup.DefaultSingleVersion,
	}

	found := false
	var lastClass outputClassification
	for _, v := range versions {
		for _, initiator := range []bool{true, false} {
			s := makeSingle(v, initiator)
			class, err := classifyOutputs(s, &tx)
			require.NoError(t, err)
			if class.toRemoteIdx >= 0 {
				found = true
				lastClass = class
				t.Logf("Matched with version=%v initiator=%v",
					v, initiator)

				break
			}
		}
		if found {
			break
		}
	}

	require.True(t, found, "to_remote output not identified for "+
		"provided data")

	// Log the results.
	printOutputClassification(lastClass, &tx)

	// Verify the logged classification.
	h.assertLogContains("Output to_remote: idx=3 amount=790968 sat")
	h.assertLogContains("Possible anchor: idx=0 amount=330 sat")
	h.assertLogContains("Possible anchor: idx=1 amount=330 sat")
	h.assertLogContains("Possible to_local/htlc: idx=2 amount=8087 sat")
}
