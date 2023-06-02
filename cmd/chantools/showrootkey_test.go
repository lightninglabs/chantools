package main

import (
	"testing"

	"github.com/lightninglabs/chantools/btc"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/stretchr/testify/require"
)

func TestShowRootKey(t *testing.T) {
	h := newHarness(t)

	// Derive the root key from the aezeed.
	show := &showRootKeyCommand{
		rootKey: &rootKey{},
	}

	t.Setenv(lnd.MnemonicEnvName, seedAezeedNoPassphrase)
	t.Setenv(lnd.PassphraseEnvName, "-")

	err := show.Execute(nil, nil)
	require.NoError(t, err)

	h.assertLogContains(rootKeyAezeed)
}

func TestShowRootKeyBIP39(t *testing.T) {
	h := newHarness(t)

	// Derive the root key from the BIP39 seed.
	show := &showRootKeyCommand{
		rootKey: &rootKey{BIP39: true},
	}

	t.Setenv(btc.BIP39MnemonicEnvName, seedBip39)
	t.Setenv(btc.BIP39PassphraseEnvName, "-")

	err := show.Execute(nil, nil)
	require.NoError(t, err)

	h.assertLogContains(rootKeyBip39)
}

func TestShowRootKeyBIP39WithPassphrase(t *testing.T) {
	h := newHarness(t)

	// Derive the root key from the BIP39 seed.
	show := &showRootKeyCommand{
		rootKey: &rootKey{BIP39: true},
	}

	t.Setenv(btc.BIP39MnemonicEnvName, seedBip39)
	t.Setenv(btc.BIP39PassphraseEnvName, testPassPhrase)

	err := show.Execute(nil, nil)
	require.NoError(t, err)

	h.assertLogContains(rootKeyBip39Passphrase)
}
