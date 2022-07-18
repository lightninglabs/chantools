package main

import (
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/guggero/chantools/lnd"
)

var sweepTimeLockManualCases = []struct {
	baseKey         string
	keyIndex        uint32
	timeLockAddr    string
	remoteRevPubKey string
}{{
	// New format with ECDH revocation root.
	baseKey: "tprv8dgoXnQWBN4CGGceRYMW495kWcrUZKZVFwMmbzpduFp1D4pi" +
		"3B2t37zTG5Fx66XWPDQYi3Q5vqDgmmZ5ffrqZ9H4s2EhJu9WaJjY3SKaWDK",
	keyIndex: 7,
	timeLockAddr: "bcrt1qf9zv4qtxh27c954rhlzg4tx58xh0vgssuu0csrlep0jdnv" +
		"lx9xesmcl5qx",
	remoteRevPubKey: "03235261ed5aaaf9fec0e91d5e1a4d17f1a2c7442f1c43806d32" +
		"c9bd34abd002a3",
}, {
	// Old format with plain private key as revocation root.
	baseKey: "tprv8dgoXnQWBN4CGGceRYMW495kWcrUZKZVFwMmbzpduFp1D4pi" +
		"3B2t37zTG5Fx66XWPDQYi3Q5vqDgmmZ5ffrqZ9H4s2EhJu9WaJjY3SKaWDK",
	keyIndex: 6,
	timeLockAddr: "bcrt1qa5rrlswxefc870k7rsza5hhqd37uytczldjk5t0vzd95u9" +
		"hs8xlsfdc3zf",
	remoteRevPubKey: "03e82cdf164ce5aba253890e066129f134ca8d7e072ce5ad55c7" +
		"21b9a13545ee04",
}}

func TestSweepTimeLockManual(t *testing.T) {
	for _, tc := range sweepTimeLockManualCases {
		// First, we need to parse the lock addr and make sure we can
		// brute force the script with the information we have. If not,
		// we can't continue anyway.
		lockScript, err := lnd.GetP2WSHScript(
			tc.timeLockAddr, &chaincfg.RegressionNetParams,
		)
		if err != nil {
			t.Fatalf("invalid time lock addr: %v", err)
		}

		baseKey, err := hdkeychain.NewKeyFromString(tc.baseKey)
		if err != nil {
			t.Fatalf("couldn't derive base key: %v", err)
		}

		revPubKeyBytes, _ := hex.DecodeString(tc.remoteRevPubKey)
		revPubKey, _ := btcec.ParsePubKey(revPubKeyBytes)

		_, _, _, _, _, err = tryKey(
			baseKey, revPubKey, defaultCsvLimit, lockScript,
			tc.keyIndex,
		)
		if err != nil {
			t.Fatalf("couldn't derive key: %v", err)
		}
	}
}
