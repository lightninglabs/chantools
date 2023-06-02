package main

import (
	"testing"

	"github.com/lightninglabs/chantools/btc"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/stretchr/testify/require"
)

const (
	testPath        = "m/123'/45'/67'/8/9"
	keyContent      = "bcrt1qnl5qfvpfcmj7y56nugpermluu46x79sfz0ku70"
	keyContentBIP39 = "bcrt1q3pae32m7jdqm5ulf80yc3n59xy4s4xm5a28ekr"
)

func TestDeriveKey(t *testing.T) {
	h := newHarness(t)

	// Derive a specific key from the serialized root key.
	derive := &deriveKeyCommand{
		Path:    testPath,
		rootKey: &rootKey{RootKey: rootKeyAezeed},
	}

	err := derive.Execute(nil, nil)
	require.NoError(t, err)

	h.assertLogContains(keyContent)
}

func TestDeriveKeyAezeedNoPassphrase(t *testing.T) {
	h := newHarness(t)

	// Derive a specific key from the serialized root key.
	derive := &deriveKeyCommand{
		Path:    testPath,
		rootKey: &rootKey{},
	}

	t.Setenv(lnd.MnemonicEnvName, seedAezeedNoPassphrase)
	t.Setenv(lnd.PassphraseEnvName, "-")

	err := derive.Execute(nil, nil)
	require.NoError(t, err)

	h.assertLogContains(keyContent)
}

func TestDeriveKeyAezeedWithPassphrase(t *testing.T) {
	h := newHarness(t)

	// Derive a specific key from the serialized root key.
	derive := &deriveKeyCommand{
		Path:    testPath,
		rootKey: &rootKey{},
	}

	t.Setenv(lnd.MnemonicEnvName, seedAezeedWithPassphrase)
	t.Setenv(lnd.PassphraseEnvName, testPassPhrase)

	err := derive.Execute(nil, nil)
	require.NoError(t, err)

	h.assertLogContains(keyContent)
}

func TestDeriveKeySeedBip39(t *testing.T) {
	h := newHarness(t)

	// Derive a specific key from the serialized root key.
	derive := &deriveKeyCommand{
		Path:    testPath,
		rootKey: &rootKey{BIP39: true},
	}

	t.Setenv(btc.BIP39MnemonicEnvName, seedBip39)
	t.Setenv(btc.BIP39PassphraseEnvName, "-")

	err := derive.Execute(nil, nil)
	require.NoError(t, err)

	h.assertLogContains(keyContentBIP39)
}
