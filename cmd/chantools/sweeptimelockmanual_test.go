package main

import (
	"bytes"
	"encoding/hex"
	"math"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/input"
	"github.com/stretchr/testify/require"
)

var sweepTimeLockManualCases = []struct {
	baseKey         string
	rootKey         string
	basePath        []uint32
	keyIndex        uint32
	timeLockAddr    string
	remoteRevPubKey string
	params          *chaincfg.Params
}{{
	// New format with ECDH revocation root.
	baseKey: "tprv8dgoXnQWBN4CGGceRYMW495kWcrUZKZVFwMmbzpduFp1D4pi" +
		"3B2t37zTG5Fx66XWPDQYi3Q5vqDgmmZ5ffrqZ9H4s2EhJu9WaJjY3SKaWDK",
	keyIndex: 7,
	timeLockAddr: "bcrt1qf9zv4qtxh27c954rhlzg4tx58xh0vgssuu0csrlep0jdnv" +
		"lx9xesmcl5qx",
	remoteRevPubKey: "03235261ed5aaaf9fec0e91d5e1a4d17f1a2c7442f1c43806d" +
		"32c9bd34abd002a3",
	params: &chaincfg.RegressionNetParams,
}, {
	// Old format with plain private key as revocation root.
	baseKey: "tprv8dgoXnQWBN4CGGceRYMW495kWcrUZKZVFwMmbzpduFp1D4pi" +
		"3B2t37zTG5Fx66XWPDQYi3Q5vqDgmmZ5ffrqZ9H4s2EhJu9WaJjY3SKaWDK",
	keyIndex: 6,
	timeLockAddr: "bcrt1qa5rrlswxefc870k7rsza5hhqd37uytczldjk5t0vzd95u9" +
		"hs8xlsfdc3zf",
	remoteRevPubKey: "03e82cdf164ce5aba253890e066129f134ca8d7e072ce5ad55" +
		"c721b9a13545ee04",
	params: &chaincfg.RegressionNetParams,
}, {
	// New format with ECDH revocation root.
	baseKey: "tprv8fCiPGhoYhWESQg3kgubCizcHo21drnP9Fa5j9fFKCmbME" +
		"ipgodofyXcf4NFhD4k55GM1Ym3JUUDonpEXcsjnyTDUMmkzMK9pCnGPH3NJ5i",
	keyIndex: 0,
	timeLockAddr: "bcrt1qmkyn0tqx6mpg5aujgjhzaw27rvvymdfc3xhgawp48zy8v" +
		"3rlw45qzmjqrr",
	remoteRevPubKey: "02dfecdc259a7e1cff36a67328ded3b4dae30369a3035e4f91" +
		"1ce7ac4a80b28e5d",
	params: &chaincfg.RegressionNetParams,
}, {
	// Old format with plain private key as revocation root. Test data
	// created with lnd v0.12.0-beta (old shachain root creation)
	baseKey: "tprv8e3Mee42NcUd2MbwxBCJyEEhvKa8KqjiDR76M7ym4DJSfZk" +
		"fDyA46XZeA4kTj8YKktWrjGBDThxxcL4HBF89jDKseu24XtugVMNsm3GhHwK",
	keyIndex: 0,
	timeLockAddr: "bcrt1qux548e45wlg9sufhgd8ldfzqrapl303g5sj7xg5w637sge" +
		"dst0wsk0xags",
	remoteRevPubKey: "03647afa9c04025e997a5b7ecd2dd949f8f60f6880a94af73a" +
		"0d4f48f166d127d1",
	params: &chaincfg.RegressionNetParams,
}, {
	// New format with ECDH revocation root but this test data was created
	// when already the old format was present, this leads to the situation
	// where the idx for the shachain root (revocation root) is equal to
	// the delay basepoint index. Normally when starting a node after
	// lnd with the version v0.13.0-beta onwards, the index is always
	// +1 compared to the delay basepoint index.
	baseKey: "tprv8e3Mee42NcUd2MbwxBCJyEEhvKa8KqjiDR76M7ym4DJSfZ" +
		"kfDyA46XZeA4kTj8YKktWrjGBDThxxcL4HBF89jDKseu24XtugVMNsm3GhHwK",
	keyIndex: 1,
	timeLockAddr: "bcrt1qsj7c97fj9xh8znlkjtg4x45xstypk5zp3kcnt5f5u6ps" +
		"rhetju2srseqrh",
	remoteRevPubKey: "0341692a025ad552c62689a630ff24d9439e3752d8e0ac5cb4" +
		"1b5e71ab2bd46d0f",
	params: &chaincfg.RegressionNetParams,
}, {
	// Anchor channel with lnd 0.18.x-beta.
	rootKey: "tprv8ZgxMBicQKsPdRiEUwMA71fkU9XoiPakhuSAEGCfcpgZxraB" +
		"eXKCfJSVo2SMAAEENrZU6x4V5gu5at6cpnasaf9oSrUh72zXaEdWDgSFVEf",
	basePath: []uint32{lnd.HardenedKey(1017), lnd.HardenedKey(1)},
	keyIndex: 3,
	timeLockAddr: "bcrt1qqm2u8pyqjc8akatdyap5qsjh7z4fuytwaehsjndt93e50l0" +
		"sup4qa2384v",
	remoteRevPubKey: "028a2199d9c64f21b59c01a5d5429fbe78b8709f0270deaf91" +
		"578f682b87a12796",
	params: &chaincfg.RegressionNetParams,
}}

func TestSweepTimeLockManual(t *testing.T) {
	for _, tc := range sweepTimeLockManualCases {
		// First, we need to parse the lock addr and make sure we can
		// brute-force the script with the information we have. If not,
		// we can't continue anyway.
		lockScript, err := lnd.GetP2WSHScript(
			tc.timeLockAddr, tc.params,
		)
		require.NoError(t, err)

		var baseKey *hdkeychain.ExtendedKey
		if tc.baseKey != "" {
			baseKey, err = hdkeychain.NewKeyFromString(tc.baseKey)
			require.NoError(t, err)
		} else {
			rootKey, err := hdkeychain.NewKeyFromString(tc.rootKey)
			require.NoError(t, err)

			baseKey, err = lnd.DeriveChildren(rootKey, tc.basePath)
			require.NoError(t, err)
		}

		revPubKeyBytes, _ := hex.DecodeString(tc.remoteRevPubKey)
		revPubKey, _ := btcec.ParsePubKey(revPubKeyBytes)

		_, _, _, _, _, err = tryKey(
			baseKey, revPubKey, 0, defaultCsvLimit, lockScript,
			tc.keyIndex, tc.keyIndex, 1000,
		)
		require.NoError(t, err)
	}
}

// TestBruteForceDelayMaxCsvTimeout makes sure the brute force loop terminates
// when the maximum CSV timeout value of math.MaxUint16 is used, which would
// overflow the loop counter if it was a uint16 as well.
func TestBruteForceDelayMaxCsvTimeout(t *testing.T) {
	delayPubKey, err := pubKeyFromHex(
		"03235261ed5aaaf9fec0e91d5e1a4d17f1a2c7442f1c43806d32c9bd34ab" +
			"d002a3",
	)
	require.NoError(t, err)

	revPubKey, err := pubKeyFromHex(
		"03e82cdf164ce5aba253890e066129f134ca8d7e072ce5ad55c721b9a135" +
			"45ee04",
	)
	require.NoError(t, err)

	// The script we're looking for uses the largest possible CSV value, so
	// the loop needs to run through the full uint16 range to find it.
	script, err := input.CommitScriptToSelf(
		math.MaxUint16, delayPubKey, revPubKey,
	)
	require.NoError(t, err)

	scriptHash, err := input.WitnessScriptHash(script)
	require.NoError(t, err)

	// A script that doesn't correspond to any CSV value in the range, to
	// make sure the loop also terminates if nothing is found at all.
	unknownScript := bytes.Repeat([]byte{0xff}, 34)

	// Runs the brute force loop in a goroutine and makes sure it returns
	// within a reasonable amount of time instead of looping forever. The
	// assertions on the result are done by the caller, which runs in the
	// goroutine of the test function itself.
	runBruteForce := func(target []byte) bruteForceResult {
		t.Helper()

		resultChan := make(chan bruteForceResult, 1)
		go func() {
			var res bruteForceResult
			res.csvTimeout, res.script, res.scriptHash, res.err =
				bruteForceDelay(
					delayPubKey, revPubKey, target, 0,
					math.MaxUint16,
				)
			resultChan <- res
		}()

		select {
		case res := <-resultChan:
			return res

		case <-time.After(time.Minute):
			require.Fail(t, "bruteForceDelay did not terminate")

			return bruteForceResult{}
		}
	}

	found := runBruteForce(scriptHash)
	require.NoError(t, found.err)
	require.EqualValues(t, math.MaxUint16, found.csvTimeout)
	require.Equal(t, script, found.script)
	require.Equal(t, scriptHash, found.scriptHash)

	notFound := runBruteForce(unknownScript)
	require.ErrorContains(t, notFound.err, "csv timeout not found")
}

// bruteForceResult holds the return values of a bruteForceDelay call.
type bruteForceResult struct {
	csvTimeout int32
	script     []byte
	scriptHash []byte
	err        error
}
