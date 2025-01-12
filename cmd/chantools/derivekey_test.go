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

func TestDeriveKeyXprv(t *testing.T) {
	h := newHarness(t)

	// Derive a specific key from xprv.
	derive := &deriveKeyCommand{
		Path: testPath,
		rootKey: &rootKey{
			RootKey: "xprv9s21ZrQH143K3QTDL4LXw2F7HEK3wJUD2nW2nR" +
				"k4stbPy6cq3jPPqjiChkVvvNKmPGJxWUtg6LnF5kejM" +
				"RNNU3TGtRBeJgk33yuGBxrMPHi",
		},
	}

	err := derive.Execute(nil, nil)
	require.NoError(t, err)

	h.assertLogContains("cQcdieZy2d1TAdCsa5MjmHJs2gdHcD7x22nDbhJyVTUa3Ax" +
		"5KB3w")
}

func TestDeriveKeyXpub(t *testing.T) {
	h := newHarness(t)

	// Derive a specific key from xpub.
	derive := &deriveKeyCommand{
		Path: "m/5/6",
		rootKey: &rootKey{
			RootKey: "xpub661MyMwAqRbcFtXgS5sYJABqqG9YLmC4Q1Rdap" +
				"9gSE8NqtwybGhePY2gZ29ESFjqJoCu1Rupje8YtGqse" +
				"fD265TMg7usUDFdp6W1EGMcet8",
		},
		Neuter: true,
	}

	err := derive.Execute(nil, nil)
	require.NoError(t, err)

	h.assertLogContains("03dc8655d58bd4fd4326863fe34bd5cdddbefaa3b042571" +
		"05eb1ab99aa05e01c2a")
}

func TestDeriveKeyXpubNoNeuter(t *testing.T) {
	h := newHarness(t)

	// Derive a specific key from xpub.
	derive := &deriveKeyCommand{
		Path: "m/5/6",
		rootKey: &rootKey{
			RootKey: "xpub661MyMwAqRbcFtXgS5sYJABqqG9YLmC4Q1Rdap" +
				"9gSE8NqtwybGhePY2gZ29ESFjqJoCu1Rupje8YtGqse" +
				"fD265TMg7usUDFdp6W1EGMcet8",
		},
	}

	err := derive.Execute(nil, nil)
	require.NoError(t, err)

	h.assertLogContains("03dc8655d58bd4fd4326863fe34bd5cdddbefaa3b042571" +
		"05eb1ab99aa05e01c2a")
}
