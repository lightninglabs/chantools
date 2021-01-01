package main

import (
	"bytes"
	"fmt"
	"io/ioutil"
	"os"
	"path"
	"regexp"
	"testing"

	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btclog"
	"github.com/lightningnetwork/lnd/chanbackup"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/stretchr/testify/require"
)

const (
	seedAezeedNoPassphrase = "abandon kangaroo tribe spell brass entry " +
		"argue buzz muffin total rug title autumn wish use bubble " +
		"alarm rent machine hockey fork slam gaze tobacco"
	seedAezeedWithPassphrase = "able pause keen exhibit duck olympic " +
		"foot donor hire omit earth ribbon rotate cruise door orbit " +
		"nephew mixture machine hockey fork scorpion shell door"
	testPassPhrase = "testnet3"
	seedBip39      = "uncover bargain diesel boss local host over divide " +
		"orient cradle good crumble"

	rootKeyAezeed = "tprv8ZgxMBicQKsPejNXQLJKe3dBBs9Zrt53EZrsBzVLQ8rZji3" +
		"hVb3wcoRvgrjvTmjPG2ixoGUUkCyC6yBEy9T5gbLdvD2a5VmJbcFd5Q9pkAs"
	rootKeyBip39 = "tprv8ZgxMBicQKsPdoVEZRN2MyzEgxGTqJepzhMc66b26zL1siLi" +
		"WRQAGh9rAgPPJuQeHWWpgcDcS45yi6KBTFeGkQMEb2RNTrP11evJcB4UVSh"
	rootKeyBip39Passphrase = ""
)

var (
	datePattern = regexp.MustCompile(
		"\\d{4}-\\d{2}-\\d{2} \\d{2}:\\d{2}:\\d{2}\\.\\d{3} ",
	)
	addressPattern = regexp.MustCompile("\\(0x[0-9a-f]{10}\\)")
)

type harness struct {
	t         *testing.T
	logBuffer *bytes.Buffer
	logger    btclog.Logger
	tempDir   string
}

func newHarness(t *testing.T) *harness {
	buf := &bytes.Buffer{}
	logBackend := btclog.NewBackend(buf)
	tempDir, err := ioutil.TempDir("", "chantools")
	require.NoError(t, err)

	h := &harness{
		t:         t,
		logBuffer: buf,
		logger:    logBackend.Logger("CHAN"),
		tempDir:   tempDir,
	}

	h.logger.SetLevel(btclog.LevelTrace)
	log = h.logger
	channeldb.UseLogger(h.logger)
	chanbackup.UseLogger(h.logger)

	os.Clearenv()
	chainParams = &chaincfg.RegressionNetParams

	return h
}

func (h *harness) getLog() string {
	return h.logBuffer.String()
}

func (h *harness) clearLog() {
	h.logBuffer.Reset()
}

func (h *harness) assertLogContains(format string, args ...interface{}) {
	h.t.Helper()

	require.Contains(h.t, h.logBuffer.String(), fmt.Sprintf(format, args...))
}

func (h *harness) assertLogEqual(a, b string) {
	// Remove all timestamps and all memory addresses from dumps as those
	// are always different.
	a = datePattern.ReplaceAllString(a, "")
	a = addressPattern.ReplaceAllString(a, "")

	b = datePattern.ReplaceAllString(b, "")
	b = addressPattern.ReplaceAllString(b, "")

	require.Equal(h.t, a, b)
}

func (h *harness) testdataFile(name string) string {
	workingDir, err := os.Getwd()
	require.NoError(h.t, err)

	return path.Join(workingDir, "testdata", name)
}

func (h *harness) tempFile(name string) string {
	return path.Join(h.tempDir, name)
}

func (h *harness) fileSize(name string) int64 {
	stat, err := os.Stat(name)
	require.NoError(h.t, err)

	return stat.Size()
}
