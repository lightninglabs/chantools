package itest

import (
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

const (
	testXPriv = "xprv9s21ZrQH143K2ZzuN99NjD7oJruBomAVbNDXzPmhHYcwf8WXCsML" +
		"63azyS1rzzfpUsLifeDkM4Q6U9PF9RP7frSGKkMfDTDiiyQjH2PUj2z"

	testMnemonic = "about wolf boost other battle asthma refuse wedding " +
		"few purchase track one smooth tunnel immune glass infant " +
		"tag manual multiply diagram orient wrist agent"
)

var (
	readTimeout    = 100 * time.Millisecond
	defaultTimeout = 5 * time.Second
)

func TestChantoolsShowRootKeyXPriv(t *testing.T) {
	proc := StartChantools(t, "showrootkey", "--rootkey", testXPriv)
	defer proc.Kill(t)

	output := proc.ReadOutputUntil(
		t, "Your BIP32 HD root key is:", defaultTimeout,
	)
	require.Contains(t, output, "Your BIP32 HD root key is: "+testXPriv)
}

func TestChantoolsShowRootKeyMnemonic(t *testing.T) {
	proc := StartChantools(t, "showrootkey")
	defer proc.Kill(t)

	go func() {
		errString, err := proc.stderr.ReadString('\n')
		log.Errorf("chantools stderr: %v, error: %v", errString, err)
	}()

	mnemonicPrompt := proc.ReadAvailableOutput(t, readTimeout)
	require.Contains(t, mnemonicPrompt, "Input your 24-word mnemonic")
	proc.WriteInput(t, testMnemonic+"\n")

	passphrasePrompt := proc.ReadAvailableOutput(t, readTimeout)
	require.Contains(
		t, passphrasePrompt, "Input your cipher seed passphrase",
	)
	proc.WriteInput(t, "\n")

	output := proc.ReadOutputUntil(
		t, "Your BIP32 HD root key is:", defaultTimeout,
	)
	require.Contains(t, output, "Your BIP32 HD root key is: "+testXPriv)
}
