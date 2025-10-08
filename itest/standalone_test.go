package itest

import (
	"testing"

	"github.com/stretchr/testify/require"
)

const (
	testXPriv = "xprv9s21ZrQH143K2ZzuN99NjD7oJruBomAVbNDXzPmhHYcwf8WXCsML" +
		"63azyS1rzzfpUsLifeDkM4Q6U9PF9RP7frSGKkMfDTDiiyQjH2PUj2z"

	testMnemonic = "about wolf boost other battle asthma refuse wedding " +
		"few purchase track one smooth tunnel immune glass infant " +
		"tag manual multiply diagram orient wrist agent"

	testIdentityKey = "03c0da201eedf786d432b6b93fc054355478aee2a97448971c" +
		"dfcfa8f13953c58c"

	testHsmSecret = "f85e33c27dc7a87c81ee1f9d8ae15c6d756e53089d75aa6dc480" +
		"3d23b4af4b2b"

	testClnIdentityKey = "026672d7c7bce3ffb9cdc7fe42c433dc78f113732d8459e" +
		"608ae301409ba1a6f05"
)

func TestChantoolsShowRootKeyXPriv(t *testing.T) {
	proc := StartChantools(t, "showrootkey", "--rootkey", testXPriv)
	defer proc.Wait(t)

	output := proc.ReadAvailableOutput(t, defaultTimeout)
	require.Contains(t, output, "Your BIP32 HD root key is: "+testXPriv)
}

func TestChantoolsShowRootKeyMnemonic(t *testing.T) {
	proc := StartChantools(t, "showrootkey")
	defer proc.Wait(t)

	mnemonicPrompt := proc.ReadAvailableOutput(t, readTimeout)
	require.Contains(t, mnemonicPrompt, "Input your 24-word mnemonic")
	proc.WriteInput(t, testMnemonic+"\n")

	passphrasePrompt := proc.ReadAvailableOutput(t, readTimeout)
	require.Contains(
		t, passphrasePrompt, "Input your cipher seed passphrase",
	)
	proc.WriteInput(t, "\n")

	output := proc.ReadAvailableOutput(t, defaultTimeout)
	require.Contains(t, output, "Your BIP32 HD root key is: "+testXPriv)
}

func TestDeriveKey(t *testing.T) {
	cmdOutput := invokeCmdDeriveKey(
		t, nil, "--identity", "--rootkey", testXPriv,
	)
	pubKey := extractRowContent(cmdOutput, rowPublicKey)
	require.Equal(t, testIdentityKey, pubKey)
}

func TestDeriveKeyCln(t *testing.T) {
	cmdOutput := invokeCmdDeriveKey(
		t, nil, "--identity", "--hsm_secret", testHsmSecret,
	)
	pubKey := extractRowContent(cmdOutput, rowPublicKey)
	require.Equal(t, testClnIdentityKey, pubKey)
}
