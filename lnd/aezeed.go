package lnd

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"syscall"
	"time"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcutil/hdkeychain"
	"github.com/lightningnetwork/lnd/aezeed"
	"golang.org/x/crypto/ssh/terminal"
)

var (
	numberDotsRegex = regexp.MustCompile("[\\d.\\-\\n\\r\\t]*")
	multipleSpaces  = regexp.MustCompile(" [ ]+")
)

func ReadAezeed(params *chaincfg.Params) (*hdkeychain.ExtendedKey, time.Time,
	error) {

	// We'll now prompt the user to enter in their 24-word mnemonic.
	fmt.Printf("Input your 24-word mnemonic separated by spaces: ")
	reader := bufio.NewReader(os.Stdin)
	mnemonicStr, err := reader.ReadString('\n')
	if err != nil {
		return nil, time.Unix(0, 0), err
	}

	// We'll trim off extra spaces, and ensure the mnemonic is all
	// lower case.
	mnemonicStr = strings.TrimSpace(mnemonicStr)
	mnemonicStr = strings.ToLower(mnemonicStr)

	// To allow the tool to also accept the copy/pasted version of the
	// backup text (which contains numbers and dots and multiple spaces),
	// we do some more cleanup with regex.
	mnemonicStr = numberDotsRegex.ReplaceAllString(mnemonicStr, "")
	mnemonicStr = multipleSpaces.ReplaceAllString(mnemonicStr, " ")
	mnemonicStr = strings.TrimSpace(mnemonicStr)

	cipherSeedMnemonic := strings.Split(mnemonicStr, " ")

	fmt.Println()

	if len(cipherSeedMnemonic) != 24 {
		return nil, time.Unix(0, 0), fmt.Errorf("wrong cipher seed "+
			"mnemonic length: got %v words, expecting %v words",
			len(cipherSeedMnemonic), 24)
	}

	// Additionally, the user may have a passphrase, that will also
	// need to be provided so the daemon can properly decipher the
	// cipher seed.
	fmt.Printf("Input your cipher seed passphrase (press enter if " +
		"your seed doesn't have a passphrase): ")
	passphrase, err := terminal.ReadPassword(int(syscall.Stdin)) // nolint
	if err != nil {
		return nil, time.Unix(0, 0), err
	}
	fmt.Println()

	var mnemonic aezeed.Mnemonic
	copy(mnemonic[:], cipherSeedMnemonic)

	// If we're unable to map it back into the ciphertext, then either the
	// mnemonic is wrong, or the passphrase is wrong.
	cipherSeed, err := mnemonic.ToCipherSeed(passphrase)
	if err != nil {
		return nil, time.Unix(0, 0), fmt.Errorf("failed to decrypt "+
			"seed with passphrase: %v", err)
	}
	rootKey, err := hdkeychain.NewMaster(cipherSeed.Entropy[:], params)
	if err != nil {
		return nil, time.Unix(0, 0), fmt.Errorf("failed to derive " +
			"master extended key")
	}
	return rootKey, cipherSeed.BirthdayTime(), nil
}
