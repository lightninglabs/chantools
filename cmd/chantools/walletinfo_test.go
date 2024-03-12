package main

import (
	"testing"

	"github.com/lightninglabs/chantools/lnd"
	"github.com/stretchr/testify/require"
)

const (
	walletContent = "03b99ab108e39e9e4cf565c1b706480180a70a4fdc4828e44c50" +
		"4530c056be5b5f"
)

func TestWalletInfo(t *testing.T) {
	h := newHarness(t)

	// Dump the wallet information.
	info := &walletInfoCommand{
		WalletDB:    h.testdataFile("wallet.db"),
		WithRootKey: true,
	}

	t.Setenv(lnd.PasswordEnvName, testPassPhrase)

	err := info.Execute(nil, nil)
	require.NoError(t, err)

	h.assertLogContains(walletContent)
	h.assertLogContains(rootKeyAezeed)
}
