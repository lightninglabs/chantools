// nolint: unparam
package itest

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"testing"
	"time"

	"github.com/btcsuite/btcd/btcec/v2/schnorr"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/txscript"
	"github.com/decred/dcrd/dcrec/secp256k1/v4"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/encoding/protojson"
)

// Local addresses for services running in the Docker Compose setup.
// See docker/.env, where they are defined.
const (
	localElectrsAddr = "http://127.0.0.1:3004"
	localDaveAddr    = "127.0.0.1:9700"
	localSnykeAddr   = "127.0.0.1:9701"
)

const (
	channelsFilePattern = "docker/node-data/chantools/%s-channels.json"
	walletFilePattern   = "docker/node-data/%s/data/chain/bitcoin/" +
		"regtest/wallet.db"
	hsmSecretFilePattern    = "docker/node-data/%s/regtest/hsm_secret"
	nodeIdentityFilePattern = "docker/node-data/chantools/identities.txt"
	nodeURIPattern          = "%s@%s"

	// rowPublicKey matches "Public key:" as well as "Node identity public
	// key:".
	rowPublicKey = "ublic key:"

	rowResults          = " Writing result to"
	rowChannelBalance   = "before fees?:"
	rowOffer            = "other party to review and sign (if they accept):"
	psbtBase64Ident     = "cHNid"
	transactionHexIdent = "02000000"
	rowSign             = "Press <enter> to continue and sign the " +
		"transaction or <ctrl+c> to abort:"
	rowPublish     = "Please publish this using any bitcoin node:"
	rowTransaction = "Transaction:"
	rowForceClose  = "Found force close transaction"
)

var (
	readTimeout    = 100 * time.Millisecond
	testTick       = 50 * time.Millisecond
	shortTimeout   = 500 * time.Millisecond
	defaultTimeout = 5 * time.Second
	longTimeout    = 30 * time.Second

	testParams = chaincfg.RegressionNetParams

	lncliOpts = &protojson.UnmarshalOptions{
		AllowPartial:   true,
		DiscardUnknown: true,
		UseHexForBytes: true,
	}

	emptyPassword = ""
)

type clnChannel struct {
	Source      string `json:"source"`
	Destination string `json:"destination"`
	ShortID     string `json:"short_channel_id"`
	AmountMsat  int64  `json:"amount_msat"`
}

type clnChannelList struct {
	Channels []clnChannel `json:"channels"`
}

func extractRowContent(row, prefix string) string {
	rex := regexp.MustCompile(
		fmt.Sprintf(`(?sm)%s\s+(.*?)\s+`, regexp.QuoteMeta(prefix)),
	)
	matches := rex.FindAllStringSubmatch(row, -1)
	for _, groups := range matches {
		return groups[1]
	}

	return ""
}

func randTaprootAddr(t *testing.T) string {
	t.Helper()

	key, err := secp256k1.GeneratePrivateKey()
	require.NoError(t, err)

	trKey := txscript.ComputeTaprootKeyNoScript(key.PubKey())

	addr, err := btcutil.NewAddressTaproot(
		schnorr.SerializePubKey(trKey), &testParams,
	)
	require.NoError(t, err)

	return addr.EncodeAddress()
}

func readChannelsJSON(t *testing.T, node string) []*lnrpc.Channel {
	t.Helper()

	contentBytes, err := os.ReadFile(fmt.Sprintf(channelsFilePattern, node))
	require.NoError(t, err)

	var resp lnrpc.ListChannelsResponse
	err = lncliOpts.Unmarshal(contentBytes, &resp)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Channels)

	return resp.Channels
}

func readNodeIdentityFromFile(t *testing.T, node string) string {
	t.Helper()

	contentBytes, err := os.ReadFile(nodeIdentityFilePattern)
	require.NoError(t, err)

	identity := extractRowContent(string(contentBytes), node+":")

	return identity
}

func readChannelsJSONCln(t *testing.T, node string) []clnChannel {
	t.Helper()

	contentBytes, err := os.ReadFile(fmt.Sprintf(channelsFilePattern, node))
	require.NoError(t, err)

	var resp clnChannelList
	err = json.Unmarshal(contentBytes, &resp)
	require.NoError(t, err)
	require.NotEmpty(t, resp.Channels)

	return resp.Channels
}

func writeJSONFile(fileName string, v any) error {
	summaryBytes, err := json.MarshalIndent(v, "", " ")
	if err != nil {
		return err
	}
	log.Infof("Writing JSON to %s", fileName)
	return os.WriteFile(fileName, summaryBytes, 0644)
}

func invokeCmdDeriveKey(t *testing.T, walletPassword *string,
	args ...string) string {

	t.Helper()

	fullArgs := append([]string{"--regtest", "derivekey"}, args...)
	proc := StartChantools(t, fullArgs...)
	defer proc.Wait(t)

	if walletPassword != nil {
		pwPrompt := proc.ReadAvailableOutput(t, readTimeout)

		require.Contains(t, pwPrompt, "Input wallet password:")
		proc.WriteInput(t, *walletPassword+"\n")

		proc.AssertNoStderr(t)
	}

	output := proc.ReadAvailableOutput(t, defaultTimeout)
	require.Contains(t, output, rowPublicKey)

	return output
}

func invokeCmdZombieRecoveryPrepareKeys(t *testing.T, walletPassword *string,
	resultsDir string, args ...string) string {

	t.Helper()

	fullArgs := append([]string{
		"--regtest", "--resultsdir", resultsDir,
		"zombierecovery", "preparekeys",
	}, args...)
	proc := StartChantools(t, fullArgs...)
	defer proc.Wait(t)

	if walletPassword != nil {
		pwPrompt := proc.ReadAvailableOutput(t, readTimeout)

		require.Contains(t, pwPrompt, "Input wallet password:")
		proc.WriteInput(t, *walletPassword+"\n")

		proc.AssertNoStderr(t)
	}

	output := proc.ReadAvailableOutput(t, defaultTimeout)
	require.Contains(t, output, rowResults)

	return output
}

func invokeCmdZombieRecoveryMakeOffer(t *testing.T, walletPassword *string,
	resultsDir string, localBalance int64, args ...string) string {

	t.Helper()

	fullArgs := append([]string{
		"--regtest", "--resultsdir", resultsDir,
		"zombierecovery", "makeoffer",
	}, args...)
	proc := StartChantools(t, fullArgs...)
	defer proc.Wait(t)

	if walletPassword != nil {
		pwPrompt := proc.ReadAvailableOutput(t, readTimeout)

		require.Contains(t, pwPrompt, "Input wallet password:")
		proc.WriteInput(t, *walletPassword+"\n")

		proc.AssertNoStderr(t)
	}

	question := proc.ReadAvailableOutput(t, defaultTimeout)
	require.Contains(t, question, rowChannelBalance)

	proc.WriteInput(t, fmt.Sprintf("%d\n", localBalance))

	output := proc.ReadAvailableOutput(t, readTimeout)

	return output
}

func invokeCmdZombieRecoverySignOffer(t *testing.T, walletPassword *string,
	resultsDir string, args ...string) string {

	t.Helper()

	fullArgs := append([]string{
		"--regtest", "--resultsdir", resultsDir,
		"zombierecovery", "signoffer",
	}, args...)
	proc := StartChantools(t, fullArgs...)
	defer proc.Wait(t)

	if walletPassword != nil {
		pwPrompt := proc.ReadAvailableOutput(t, readTimeout)

		require.Contains(t, pwPrompt, "Input wallet password:")
		proc.WriteInput(t, *walletPassword+"\n")

		proc.AssertNoStderr(t)
	}

	question := proc.ReadAvailableOutput(t, defaultTimeout)
	require.Contains(t, question, rowSign)

	proc.WriteInput(t, "\n")

	output := proc.ReadAvailableOutput(t, readTimeout)

	return output
}

func invokeCmdSweepRemoteClosed(t *testing.T, walletPassword *string,
	resultsDir, apiURL string, args ...string) string {

	t.Helper()

	fullArgs := append([]string{
		"--regtest", "--resultsdir", resultsDir,
		"sweepremoteclosed", "--apiurl", apiURL,
	}, args...)
	proc := StartChantools(t, fullArgs...)
	defer proc.Wait(t)

	if walletPassword != nil {
		pwPrompt := proc.ReadAvailableOutput(t, readTimeout)

		require.Contains(t, pwPrompt, "Input wallet password:")
		proc.WriteInput(t, *walletPassword+"\n")

		proc.AssertNoStderr(t)
	}

	output := proc.ReadAvailableOutput(t, longTimeout)
	return output
}

func invokeCmdTriggerForceClose(t *testing.T, walletPassword *string,
	resultsDir, apiURL string, args ...string) string {

	t.Helper()

	fullArgs := append([]string{
		"--regtest", "--resultsdir", resultsDir,
		"triggerforceclose", "--apiurl", apiURL,
	}, args...)
	proc := StartChantools(t, fullArgs...)
	defer proc.Wait(t)

	if walletPassword != nil {
		pwPrompt := proc.ReadAvailableOutput(t, readTimeout)

		require.Contains(t, pwPrompt, "Input wallet password:")
		proc.WriteInput(t, *walletPassword+"\n")

		proc.AssertNoStderr(t)
	}

	output := proc.ReadAvailableOutput(t, longTimeout)
	return output
}

func getNodeIdentityKey(t *testing.T, node string) string {
	t.Helper()

	walletDbPath := fmt.Sprintf(walletFilePattern, node)
	cmdOutput := invokeCmdDeriveKey(
		t, &emptyPassword, "--identity", "--walletdb", walletDbPath,
	)
	pubKey := extractRowContent(cmdOutput, rowPublicKey)
	require.Len(
		t, pubKey, hex.EncodedLen(secp256k1.PubKeyBytesLenCompressed),
	)

	return pubKey
}

func readHsmSecret(t *testing.T, node string) string {
	t.Helper()

	hsmSecretPath := fmt.Sprintf(hsmSecretFilePattern, node)
	contentBytes, err := os.ReadFile(hsmSecretPath)
	require.NoError(t, err)
	require.Len(t, contentBytes, 32)

	return hex.EncodeToString(contentBytes)
}

func getNodeIdentityKeyCln(t *testing.T, node string) string {
	t.Helper()

	cmdOutput := invokeCmdDeriveKey(
		t, nil, "--identity", "--hsm_secret", readHsmSecret(t, node),
	)
	pubKey := extractRowContent(cmdOutput, rowPublicKey)
	require.Len(
		t, pubKey, hex.EncodedLen(secp256k1.PubKeyBytesLenCompressed),
	)

	return pubKey
}

func getZombiePreparedKeys(t *testing.T, node, tempDir, matchFile,
	payoutAddr string) string {

	t.Helper()

	walletDbPath := fmt.Sprintf(walletFilePattern, node)
	cmdOutput := invokeCmdZombieRecoveryPrepareKeys(
		t, &emptyPassword, tempDir, "--match_file", matchFile,
		"--payout_addr", payoutAddr, "--walletdb", walletDbPath,
	)
	resultFile := extractRowContent(cmdOutput, rowResults)
	require.Contains(t, resultFile, "preparedkeys")

	return resultFile
}

func getZombiePreparedKeysCln(t *testing.T, node, tempDir, matchFile,
	payoutAddr string) string {

	t.Helper()

	cmdOutput := invokeCmdZombieRecoveryPrepareKeys(
		t, nil, tempDir, "--match_file", matchFile,
		"--payout_addr", payoutAddr,
		"--hsm_secret", readHsmSecret(t, node),
	)
	resultFile := extractRowContent(cmdOutput, rowResults)
	require.Contains(t, resultFile, "preparedkeys")

	return resultFile
}

func getZombieMakeOffer(t *testing.T, node, tempDir, keys1File,
	keys2File string, localBalance int64) string {

	t.Helper()

	walletDbPath := fmt.Sprintf(walletFilePattern, node)
	cmdOutput := invokeCmdZombieRecoveryMakeOffer(
		t, &emptyPassword, tempDir, localBalance,
		"--node1_keys", keys1File, "--node2_keys", keys2File,
		"--walletdb", walletDbPath,
	)
	psbt := extractRowContent(cmdOutput, rowOffer)
	require.Contains(t, psbt, psbtBase64Ident)

	return psbt
}

func getZombieMakeOfferCln(t *testing.T, node, tempDir, keys1File,
	keys2File string, localBalance int64) string {

	t.Helper()

	cmdOutput := invokeCmdZombieRecoveryMakeOffer(
		t, nil, tempDir, localBalance,
		"--node1_keys", keys1File, "--node2_keys", keys2File,
		"--hsm_secret", readHsmSecret(t, node),
	)
	psbt := extractRowContent(cmdOutput, rowOffer)
	require.Contains(t, psbt, psbtBase64Ident)

	return psbt
}

func getZombieSignOffer(t *testing.T, node, tempDir, psbt string) string {
	t.Helper()

	walletDbPath := fmt.Sprintf(walletFilePattern, node)
	cmdOutput := invokeCmdZombieRecoverySignOffer(
		t, &emptyPassword, tempDir,
		"--psbt", psbt, "--walletdb", walletDbPath,
	)
	txHex := extractRowContent(cmdOutput, rowPublish)
	require.Contains(t, txHex, transactionHexIdent)

	return txHex
}

func getZombieSignOfferCln(t *testing.T, node, tempDir, peerIdentity,
	psbt string) string {

	t.Helper()

	cmdOutput := invokeCmdZombieRecoverySignOffer(
		t, nil, tempDir, "--remote_peer", peerIdentity, "--psbt", psbt,
		"--hsm_secret", readHsmSecret(t, node),
	)
	txHex := extractRowContent(cmdOutput, rowPublish)
	require.Contains(t, txHex, transactionHexIdent)

	return txHex
}

func getSweepRemoteClosed(t *testing.T, node, tempDir, apiURL,
	sweepAddr string) string {

	t.Helper()

	walletDbPath := fmt.Sprintf(walletFilePattern, node)
	cmdOutput := invokeCmdSweepRemoteClosed(
		t, &emptyPassword, tempDir, apiURL, "--sweepaddr", sweepAddr,
		"--recoverywindow", "10", "--walletdb", walletDbPath,
	)
	txHex := extractRowContent(cmdOutput, rowTransaction)
	require.Contains(t, txHex, transactionHexIdent)

	return txHex
}

func getSweepRemoteClosedCln(t *testing.T, node, tempDir, apiURL, peerIdentity,
	sweepAddr string) string {

	t.Helper()

	cmdOutput := invokeCmdSweepRemoteClosed(
		t, nil, tempDir, apiURL, "--sweepaddr", sweepAddr,
		"--recoverywindow", "10", "--peers", peerIdentity,
		"--hsm_secret", readHsmSecret(t, node),
	)
	txHex := extractRowContent(cmdOutput, rowTransaction)
	require.Contains(t, txHex, transactionHexIdent)

	return txHex
}

func getTriggerForceClose(t *testing.T, node, tempDir, apiURL, peerURI,
	channelPoint string) string {

	t.Helper()

	walletDbPath := fmt.Sprintf(walletFilePattern, node)
	cmdOutput := invokeCmdTriggerForceClose(
		t, &emptyPassword, tempDir, apiURL, "--peer", peerURI,
		"--channel_point", channelPoint,
		"--walletdb", walletDbPath,
	)
	txid := extractRowContent(cmdOutput, rowForceClose)
	require.Len(t, txid, hex.EncodedLen(sha256.Size))

	return txid
}
