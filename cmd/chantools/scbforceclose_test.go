package main

import (
	"bytes"
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightningnetwork/lnd/chanbackup"
	"github.com/lightningnetwork/lnd/keychain"
)

// TestClassifyOutputs_RealData verifies we can identify the to_remote output
// using lnwallet.CommitScriptToRemote with real world data provided.
func TestClassifyOutputs_RealData(t *testing.T) {
	// Remote payment basepoint (compressed) provided by user.
	remoteHex := "029e5f4d86d9d6c845fbcf37b09ac7d59c25c19932ab34a2757e8" +
		"ea88437a876c3"

	remoteBytes, err := hex.DecodeString(remoteHex)
	if err != nil {
		t.Fatalf("decode remote pubkey: %v", err)
	}
	remoteKey, err := btcec.ParsePubKey(remoteBytes)
	if err != nil {
		t.Fatalf("parse remote pubkey: %v", err)
	}

	// Example transaction hex from a real world channel.
	txHex := "020000000001011f644a3f04139c2c3b1036f9deb924f7c8101e5825a" +
		"2bf4a379579beea24bf320100000000b2448780044a01000000000000220" +
		"0202661eee6d24eaf71079b96f8df4dd88aa6280b61845dacdb10d8b0bc" +
		"c51257af4a0100000000000022002074bcb8019840e0ac7abb16be6c840" +
		"8fbbebd519cb86193965b33e8e69648865e971f0000000000002200209a1" +
		"c8e727820d673859049f9305c02c39eb0a718f9219dc2e48a2621243d7dc" +
		"8b8110c00000000002200205a596aa125a8a39e73f70dcf279cb06295ee" +
		"d49950c9e1f239b47ce41ab0e9320400483045022100ef18b0fe8d34f21" +
		"ef13316d03cbb72445b61033489a8df81f163ebd60f430637022075a25a" +
		"a0dc0a08e361540bd831430fc816b0a4ca9ca0169fb95de4a64c297cde0" +
		"1483045022100f8d7b5eee968157f0e06a65c389b6d1f5ca68a3189440b7" +
		"638ab341c5ac77fdd022069db71847c48b1f762242b99b2fa254b1bce8f4" +
		"4a160293fe3b36ed2d2e32f650147522103ae9df242881bb10a2400e781" +
		"2fc8cfe437f0f869538584d39d96f52cb2dbaf622103e71742ef40d13688" +
		"4a1f7368fb096cc5897fd697b41a3b481def37b60188c49152aebf573f20"

	txBytes, err := hex.DecodeString(txHex)
	if err != nil {
		t.Fatalf("decode tx hex: %v", err)
	}
	var tx wire.MsgTx
	if err := tx.Deserialize(bytes.NewReader(txBytes)); err != nil {
		t.Fatalf("deserialize tx: %v", err)
	}

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
			class := classifyOutputs(s, &tx)
			if class.ToRemoteIdx >= 0 {
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

	if !found {
		t.Fatalf("to_remote output not identified for provided data")
	}

	t.Logf(
		"to_remote: idx=%d amount=%d sat", lastClass.ToRemoteIdx,
		lastClass.ToRemoteAmt,
	)
	if len(lastClass.ToRemotePkScript) > 0 {
		t.Logf(
			"to_remote PkScript (hex): %x",
			lastClass.ToRemotePkScript,
		)
	}
	for _, idx := range lastClass.AnchorIdxs {
		t.Logf(
			"possible anchor: idx=%d amount=%d sat", idx,
			tx.TxOut[idx].Value,
		)
	}
	for _, idx := range lastClass.OtherIdxs {
		t.Logf(
			"possible to_local/htlc: idx=%d amount=%d sat", idx,
			tx.TxOut[idx].Value,
		)
	}
}
