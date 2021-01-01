package main

import (
	"github.com/guggero/chantools/btc"
	"os"
	"testing"

	"github.com/guggero/chantools/lnd"
	"github.com/stretchr/testify/require"
)

func TestShowRootKey(t *testing.T) {
	h := newHarness(t)

	// Derive the root key from the aezeed.
	show := &showRootKeyCommand{
		rootKey: &rootKey{},
	}

	err := os.Setenv(lnd.MnemonicEnvName, seedAezeedNoPassphrase)
	require.NoError(t, err)
	err = os.Setenv(lnd.PassphraseEnvName, "-")
	require.NoError(t, err)

	err = show.Execute(nil, nil)
	require.NoError(t, err)

	h.assertLogContains(rootKeyAezeed)
}

func TestShowRootKeyBIP39(t *testing.T) {
	h := newHarness(t)

	// Derive the root key from the BIP39 seed.
	show := &showRootKeyCommand{
		rootKey: &rootKey{BIP39: true},
	}

	err := os.Setenv(btc.BIP39MnemonicEnvName, seedBip39)
	require.NoError(t, err)
	err = os.Setenv(btc.BIP39PassphraseEnvName, "-")
	require.NoError(t, err)

	err = show.Execute(nil, nil)
	require.NoError(t, err)

	h.assertLogContains(rootKeyBip39)
}

func TestShowRootKeyBIP39WithPassphre(t *testing.T) {
	h := newHarness(t)

	// Derive the root key from the BIP39 seed.
	show := &showRootKeyCommand{
		rootKey: &rootKey{BIP39: true},
	}

	err := os.Setenv(btc.BIP39MnemonicEnvName, seedBip39)
	require.NoError(t, err)
	err = os.Setenv(btc.BIP39PassphraseEnvName, testPassPhrase)
	require.NoError(t, err)

	err = show.Execute(nil, nil)
	require.NoError(t, err)

	h.assertLogContains(rootKeyBip39Passphrase)
}