package main

import (
	"encoding/hex"
	"testing"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightninglabs/pool/poolscript"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/stretchr/testify/require"
)

type testAccount struct {
	name      string
	rootKey   string
	pkScript  string
	minExpiry uint32
}

var (
	auctioneerKeyBytes, _ = hex.DecodeString(
		"0353c7c0d3258c4957331b86af335568232e9af8df61330cee3a7488b61c" +
			"f6c298",
	)
	auctioneerKey, _ = btcec.ParsePubKey(auctioneerKeyBytes)

	testAccounts = []testAccount{{
		name: "regtest taproot (v1)",
		rootKey: "tprv8ZgxMBicQKsPdkvdLKn7HG2hhZ9Ewsgze1Yj3KDEcvb6H5U" +
			"519UtfoPPP3hYVgFTn7hXmvE41qaugbaYiZN8wM1HoQHhs3AzSwg" +
			"xGYdD8gM",
		pkScript: "512001e8d17b83358476534aae4eae2062ea9025dfd858cd81" +
			"7bac5f439969da92a6",
		minExpiry: 1600,
	}, {
		name: "regtest taproot (v2)",
		rootKey: "tprv8ZgxMBicQKsPdkvdLKn7HG2hhZ9Ewsgze1Yj3KDEcvb6H5U" +
			"519UtfoPPP3hYVgFTn7hXmvE41qaugbaYiZN8wM1HoQHhs3AzSwg" +
			"xGYdD8gM",
		pkScript: "51209dfee24b87f5c35d5a310496a64fab70641bd03d40d5cc" +
			"3720f6061f7435778a",
		minExpiry: 2060,
	}, {
		name: "regtest segwit (v0)",
		rootKey: "tprv8ZgxMBicQKsPdkvdLKn7HG2hhZ9Ewsgze1Yj3KDEcvb6H5U" +
			"519UtfoPPP3hYVgFTn7hXmvE41qaugbaYiZN8wM1HoQHhs3AzSwg" +
			"xGYdD8gM",
		pkScript: "00201acfd449370aca0f744141bc6fe1f9fe326aa57a9cd35f" +
			"bc2f8f15af4c0f4597",
		minExpiry: 1600,
	}}
)

func TestClosePoolAccount(t *testing.T) {
	t.Parallel()

	path := []uint32{
		lnd.HardenedKeyStart + uint32(keychain.BIP0043Purpose),
		lnd.HardenedKeyStart + chaincfg.RegressionNetParams.HDCoinType,
		lnd.HardenedKeyStart + uint32(poolscript.AccountKeyFamily),
		0,
	}
	const (
		maxBlocks    = 50
		maxAccounts  = 5
		maxBatchKeys = 10
	)

	for _, tc := range testAccounts {
		tc := tc

		t.Run(tc.name, func(tt *testing.T) {
			tt.Parallel()

			extendedKey, err := hdkeychain.NewKeyFromString(
				tc.rootKey,
			)
			require.NoError(tt, err)
			accountBaseKey, err := lnd.DeriveChildren(
				extendedKey, path,
			)
			require.NoError(tt, err)
			targetScriptBytes, err := hex.DecodeString(tc.pkScript)
			require.NoError(tt, err)

			acct, err := bruteForceAccountScript(
				accountBaseKey, auctioneerKey, tc.minExpiry,
				tc.minExpiry+maxBlocks, maxAccounts,
				maxBatchKeys, targetScriptBytes,
			)
			require.NoError(tt, err)
			t.Logf("Found account: %v", acct)
		})
	}
}
