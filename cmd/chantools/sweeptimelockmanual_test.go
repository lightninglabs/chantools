package main

import (
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/stretchr/testify/require"
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
	remoteRevPubKey: "03235261ed5aaaf9fec0e91d5e1a4d17f1a2c7442f1c43806d" +
		"32c9bd34abd002a3",
}, {
	// Old format with plain private key as revocation root.
	baseKey: "tprv8dgoXnQWBN4CGGceRYMW495kWcrUZKZVFwMmbzpduFp1D4pi" +
		"3B2t37zTG5Fx66XWPDQYi3Q5vqDgmmZ5ffrqZ9H4s2EhJu9WaJjY3SKaWDK",
	keyIndex: 6,
	timeLockAddr: "bcrt1qa5rrlswxefc870k7rsza5hhqd37uytczldjk5t0vzd95u9" +
		"hs8xlsfdc3zf",
	remoteRevPubKey: "03e82cdf164ce5aba253890e066129f134ca8d7e072ce5ad55" +
		"c721b9a13545ee04",
}, {
	// New format with ECDH revocation root.
	baseKey: "tprv8fCiPGhoYhWESQg3kgubCizcHo21drnP9Fa5j9fFKCmbME" +
		"ipgodofyXcf4NFhD4k55GM1Ym3JUUDonpEXcsjnyTDUMmkzMK9pCnGPH3NJ5i",
	keyIndex: 0,
	timeLockAddr: "bcrt1qmkyn0tqx6mpg5aujgjhzaw27rvvymdfc3xhgawp48zy8v" +
		"3rlw45qzmjqrr",
	remoteRevPubKey: "02dfecdc259a7e1cff36a67328ded3b4dae30369a3035e4f91" +
		"1ce7ac4a80b28e5d",
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
}, {
	// New format with ECDH revocation root but this test data was created
	// when already the old format was present, this leads to the situation
	// where the idx for the shachain root (revocation root) is equal to
	// the delay basepoint index. Normally when starting a node after
	// lnd with the version v0.13.0-beta onwords, the index is always
	// +1 compared to the delay basepoint index.
	baseKey: "tprv8e3Mee42NcUd2MbwxBCJyEEhvKa8KqjiDR76M7ym4DJSfZ" +
		"kfDyA46XZeA4kTj8YKktWrjGBDThxxcL4HBF89jDKseu24XtugVMNsm3GhHwK",
	keyIndex: 1,
	timeLockAddr: "bcrt1qsj7c97fj9xh8znlkjtg4x45xstypk5zp3kcnt5f5u6ps" +
		"rhetju2srseqrh",
	remoteRevPubKey: "0341692a025ad552c62689a630ff24d9439e3752d8e0ac5cb4" +
		"1b5e71ab2bd46d0f",
}}

func TestSweepTimeLockManual(t *testing.T) {
	for _, tc := range sweepTimeLockManualCases {
		// First, we need to parse the lock addr and make sure we can
		// brute force the script with the information we have. If not,
		// we can't continue anyway.
		lockScript, err := lnd.GetP2WSHScript(
			tc.timeLockAddr, &chaincfg.RegressionNetParams,
		)
		require.NoError(t, err)

		baseKey, err := hdkeychain.NewKeyFromString(tc.baseKey)
		require.NoError(t, err)

		revPubKeyBytes, _ := hex.DecodeString(tc.remoteRevPubKey)
		revPubKey, _ := btcec.ParsePubKey(revPubKeyBytes)

		_, _, _, _, _, err = tryKey(
			baseKey, revPubKey, defaultCsvLimit, lockScript,
			tc.keyIndex, 500,
		)
		require.NoError(t, err)
	}
}
