package main

import (
	"bytes"
	_ "embed"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"regexp"
	"slices"
	"strings"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightninglabs/chantools/btc"
	"github.com/lightninglabs/chantools/cln"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/fn/v2"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lncfg"
	"github.com/lightningnetwork/lnd/lnrpc"
	"github.com/lightningnetwork/lnd/lnwallet/chainfee"
	"github.com/spf13/cobra"
)

//go:embed sweepremoteclosed_ancient.json
var ancientChannelPoints []byte

const (
	sweepRemoteClosedDefaultRecoveryWindow = 200
	sweepDustLimit                         = 600
)

type sweepRemoteClosedCommand struct {
	RecoveryWindow uint32
	APIURL         string
	Publish        bool
	SweepAddr      string
	FeeRate        uint32

	HsmSecret    string
	PeerPubKeys  string
	KnownOutputs string

	rootKey *rootKey
	cmd     *cobra.Command
}

func newSweepRemoteClosedCommand() *cobra.Command {
	cc := &sweepRemoteClosedCommand{}
	cc.cmd = &cobra.Command{
		Use: "sweepremoteclosed",
		Short: "Go through all the addresses that could have funds of " +
			"channels that were force-closed by the remote party. " +
			"A public block explorer is queried for each address " +
			"and if any balance is found, all funds are swept to " +
			"a given address",
		Long: `This command helps users sweep funds that are in 
outputs of channels that were force-closed by the remote party. This command
only needs to be used if no channel.backup file is available. By manually
contacting the remote peers and asking them to force-close the channels, the
funds can be swept after the force-close transaction was confirmed.

Supported remote force-closed channel types are:
 - STATIC_REMOTE_KEY (a.k.a. tweakless channels)
 - ANCHOR (a.k.a. anchor output channels)
 - SIMPLE_TAPROOT (a.k.a. simple taproot channels)
`,
		Example: `chantools sweepremoteclosed \
	--recoverywindow 300 \
	--feerate 20 \
	--sweepaddr bc1q..... \
  	--publish`,
		RunE: cc.Execute,
	}
	cc.cmd.Flags().Uint32Var(
		&cc.RecoveryWindow, "recoverywindow",
		sweepRemoteClosedDefaultRecoveryWindow, "number of keys to "+
			"scan per derivation path",
	)
	cc.cmd.Flags().StringVar(
		&cc.APIURL, "apiurl", defaultAPIURL, "API URL to use (must "+
			"be esplora compatible)",
	)
	cc.cmd.Flags().BoolVar(
		&cc.Publish, "publish", false, "publish sweep TX to the chain "+
			"API instead of just printing the TX",
	)
	cc.cmd.Flags().StringVar(
		&cc.SweepAddr, "sweepaddr", "", "address to recover the funds "+
			"to; specify '"+lnd.AddressDeriveFromWallet+"' to "+
			"derive a new address from the seed automatically",
	)
	cc.cmd.Flags().Uint32Var(
		&cc.FeeRate, "feerate", defaultFeeSatPerVByte, "fee rate to "+
			"use for the sweep transaction in sat/vByte",
	)

	cc.cmd.Flags().StringVar(
		&cc.HsmSecret, "hsm_secret", "", "the hex encoded HSM secret "+
			"to use for deriving the multisig keys for a CLN "+
			"node; obtain by running 'xxd -p -c32 "+
			"~/.lightning/bitcoin/hsm_secret'",
	)
	cc.cmd.Flags().StringVar(
		&cc.PeerPubKeys, "peers", "", "comma separated list of "+
			"hex encoded public keys of the remote peers "+
			"to recover funds from, only required when using "+
			"--hsm_secret to derive the keys; can also be a file "+
			"name to a file that contains the public keys, one "+
			"per line",
	)
	cc.cmd.Flags().StringVar(
		&cc.KnownOutputs, "known_outputs", "", "a comma separated "+
			"list of known output addresses to use for matching "+
			"against, instead of querying the API; can also be "+
			"a file name to a file that contains the known "+
			"outputs, one per line",
	)

	cc.rootKey = newRootKey(cc.cmd, "sweeping the wallet")

	return cc.cmd
}

func (c *sweepRemoteClosedCommand) Execute(_ *cobra.Command, _ []string) error {
	// Make sure sweep addr is set.
	err := lnd.CheckAddress(
		c.SweepAddr, chainParams, true, "sweep", lnd.AddrTypeP2WKH,
		lnd.AddrTypeP2TR,
	)
	if err != nil {
		return err
	}

	// Set default values.
	if c.RecoveryWindow == 0 {
		c.RecoveryWindow = sweepRemoteClosedDefaultRecoveryWindow
	}
	if c.FeeRate == 0 {
		c.FeeRate = defaultFeeSatPerVByte
	}

	var (
		signer       lnd.ChannelSigner
		estimator    input.TxWeightEstimator
		knownOutputs []string
		sweepScript  []byte
		targets      []*targetAddr
	)

	if c.KnownOutputs != "" {
		knownOutputs, err = listOrFile(c.KnownOutputs)
		if err != nil {
			return fmt.Errorf("error reading known outputs: %w",
				err)
		}

		for _, output := range knownOutputs {
			_, err = lnd.ParseAddress(output, chainParams)
			if err != nil {
				return fmt.Errorf("error parsing known output "+
					"address %s: %w", output, err)
			}
		}

		log.Infof("Using %d known outputs for matching.",
			len(knownOutputs))
	}

	switch {
	case c.HsmSecret != "":
		secretBytes, err := hex.DecodeString(c.HsmSecret)
		if err != nil {
			return fmt.Errorf("error decoding HSM secret: %w", err)
		}

		var hsmSecret [32]byte
		copy(hsmSecret[:], secretBytes)

		if c.PeerPubKeys == "" {
			return errors.New("invalid peer public keys, must be " +
				"a comma separated list of hex encoded " +
				"public keys or a file name")
		}

		var pubKeys []*btcec.PublicKey
		hexPubKeys, err := listOrFile(c.PeerPubKeys)
		if err != nil {
			return fmt.Errorf("error reading peer public keys: %w",
				err)
		}
		for _, pubKeyHex := range hexPubKeys {
			pkHex, err := hex.DecodeString(pubKeyHex)
			if err != nil {
				return fmt.Errorf("error decoding peer "+
					"public key hex %s: %w", pubKeyHex, err)
			}

			pk, err := btcec.ParsePubKey(pkHex)
			if err != nil {
				return fmt.Errorf("error parsing peer public "+
					"key hex %s: %w", pubKeyHex, err)
			}

			pubKeys = append(pubKeys, pk)
		}

		log.Infof("Using %d peer public keys for recovery.",
			len(pubKeys))

		signer = &cln.Signer{
			HsmSecret: hsmSecret,
		}

		targets, err = findTargetsCln(
			hsmSecret, pubKeys, c.APIURL, c.RecoveryWindow,
			knownOutputs,
		)
		if err != nil {
			return fmt.Errorf("error finding targets: %w", err)
		}

		sweepScript, err = lnd.CheckAndEstimateAddress(
			c.SweepAddr, chainParams, &estimator, "sweep",
		)
		if err != nil {
			return err
		}

	default:
		extendedKey, err := c.rootKey.read()
		if err != nil {
			return fmt.Errorf("error reading root key: %w", err)
		}

		signer = &lnd.Signer{
			ExtendedKey: extendedKey,
			ChainParams: chainParams,
		}

		targets, err = findTargetsLnd(
			extendedKey, c.APIURL, c.RecoveryWindow, knownOutputs,
		)
		if err != nil {
			return fmt.Errorf("error finding targets: %w", err)
		}

		sweepScript, err = lnd.PrepareWalletAddress(
			c.SweepAddr, chainParams, &estimator, extendedKey,
			"sweep",
		)
		if err != nil {
			return err
		}
	}

	return sweepRemoteClosed(
		signer, &estimator, sweepScript, targets,
		newExplorerAPI(c.APIURL), c.FeeRate, c.Publish,
	)
}

type targetAddr struct {
	addr       btcutil.Address
	keyDesc    *keychain.KeyDescriptor
	tweak      []byte
	vouts      []*btc.Vout
	script     []byte
	scriptTree *input.CommitScriptTree
}

func findTargetsLnd(extendedKey *hdkeychain.ExtendedKey, apiURL string,
	recoveryWindow uint32, knownOutputs []string) ([]*targetAddr, error) {

	var (
		targets []*targetAddr
		api     = newExplorerAPI(apiURL)
	)
	for index := range recoveryWindow {
		path := fmt.Sprintf("m/1017'/%d'/%d'/0/%d",
			chainParams.HDCoinType, keychain.KeyFamilyPaymentBase,
			index)
		parsedPath, err := lnd.ParsePath(path)
		if err != nil {
			return nil, fmt.Errorf("error parsing path: %w", err)
		}

		hdKey, err := lnd.DeriveChildren(extendedKey, parsedPath)
		if err != nil {
			return nil, fmt.Errorf("eror deriving children: %w",
				err)
		}

		privKey, err := hdKey.ECPrivKey()
		if err != nil {
			return nil, fmt.Errorf("could not derive private "+
				"key: %w", err)
		}

		foundTargets, err := queryAddressBalances(
			privKey.PubKey(), &keychain.KeyDescriptor{
				PubKey: privKey.PubKey(),
				KeyLocator: keychain.KeyLocator{
					Family: keychain.KeyFamilyPaymentBase,
					Index:  index,
				},
			}, api, knownOutputs,
		)
		if err != nil {
			return nil, fmt.Errorf("could not query API for "+
				"addresses with funds: %w", err)
		}
		targets = append(targets, foundTargets...)
	}

	// Also check if there are any funds in channels with the initial,
	// tweaked channel type that requires a channel point.
	ancientChannelTargets, err := checkAncientChannelPoints(
		api, recoveryWindow, extendedKey,
	)
	if err != nil && !errors.Is(err, errAddrNotFound) {
		return nil, fmt.Errorf("could not check ancient channel "+
			"points: %w", err)
	}

	if len(ancientChannelTargets) > 0 {
		targets = append(targets, ancientChannelTargets...)
	}

	return targets, nil
}

func findTargetsCln(hsmSecret [32]byte, pubKeys []*btcec.PublicKey,
	apiURL string, recoveryWindow uint32,
	knownOutputs []string) ([]*targetAddr, error) {

	var (
		targets []*targetAddr
		api     = newExplorerAPI(apiURL)
	)
	for _, pubKey := range pubKeys {
		for index := range recoveryWindow {
			desc := &keychain.KeyDescriptor{
				PubKey: pubKey,
				KeyLocator: keychain.KeyLocator{
					Family: keychain.KeyFamilyPaymentBase,
					Index:  index,
				},
			}
			_, privKey, err := cln.DeriveKeyPair(hsmSecret, desc)
			if err != nil {
				return nil, fmt.Errorf("could not derive "+
					"private key: %w", err)
			}

			foundTargets, err := queryAddressBalances(
				privKey.PubKey(), desc, api, knownOutputs,
			)
			if err != nil {
				return nil, fmt.Errorf("could not query API "+
					"for addresses with funds: %w", err)
			}
			targets = append(targets, foundTargets...)
		}
	}

	log.Infof("Found %d addresses with funds to sweep.", len(targets))

	return targets, nil
}

func sweepRemoteClosed(signer lnd.ChannelSigner,
	estimator *input.TxWeightEstimator, sweepScript []byte,
	targets []*targetAddr, api *btc.ExplorerAPI, feeRate uint32,
	publish bool) error {

	// Create estimator and transaction template.
	var (
		signDescs        []*input.SignDescriptor
		sweepTx          = wire.NewMsgTx(2)
		totalOutputValue = uint64(0)
		prevOutFetcher   = txscript.NewMultiPrevOutFetcher(nil)
	)

	// Add all found target outputs.
	for _, target := range targets {
		for _, vout := range target.vouts {
			totalOutputValue += vout.Value

			txHash, err := chainhash.NewHashFromStr(
				vout.Outspend.Txid,
			)
			if err != nil {
				return fmt.Errorf("error parsing tx hash: %w",
					err)
			}
			pkScript, err := lnd.GetWitnessAddrScript(
				target.addr, chainParams,
			)
			if err != nil {
				return fmt.Errorf("error getting pk script: %w",
					err)
			}

			prevOutPoint := wire.OutPoint{
				Hash:  *txHash,
				Index: uint32(vout.Outspend.Vin),
			}
			prevTxOut := &wire.TxOut{
				PkScript: pkScript,
				Value:    int64(vout.Value),
			}
			prevOutFetcher.AddPrevOut(prevOutPoint, prevTxOut)
			txIn := &wire.TxIn{
				PreviousOutPoint: prevOutPoint,
				Sequence:         wire.MaxTxInSequenceNum,
			}
			sweepTx.TxIn = append(sweepTx.TxIn, txIn)
			inputIndex := len(sweepTx.TxIn) - 1

			var signDesc *input.SignDescriptor
			switch target.addr.(type) {
			case *btcutil.AddressWitnessPubKeyHash:
				estimator.AddP2WKHInput()

				signDesc = &input.SignDescriptor{
					KeyDesc:           *target.keyDesc,
					WitnessScript:     target.script,
					SingleTweak:       target.tweak,
					Output:            prevTxOut,
					HashType:          txscript.SigHashAll,
					PrevOutputFetcher: prevOutFetcher,
					InputIndex:        inputIndex,
				}

			case *btcutil.AddressWitnessScriptHash:
				estimator.AddWitnessInput(
					input.ToRemoteConfirmedWitnessSize,
				)
				txIn.Sequence = 1

				signDesc = &input.SignDescriptor{
					KeyDesc:           *target.keyDesc,
					WitnessScript:     target.script,
					Output:            prevTxOut,
					HashType:          txscript.SigHashAll,
					PrevOutputFetcher: prevOutFetcher,
					InputIndex:        inputIndex,
				}

			case *btcutil.AddressTaproot:
				estimator.AddWitnessInput(
					input.TaprootToRemoteWitnessSize,
				)
				txIn.Sequence = 1

				tree := target.scriptTree
				controlBlock, err := tree.CtrlBlockForPath(
					input.ScriptPathSuccess,
				)
				if err != nil {
					return err
				}
				controlBlockBytes, err := controlBlock.ToBytes()
				if err != nil {
					return err
				}

				script := tree.SettleLeaf.Script
				signMethod := input.TaprootScriptSpendSignMethod
				signDesc = &input.SignDescriptor{
					KeyDesc:           *target.keyDesc,
					WitnessScript:     script,
					Output:            prevTxOut,
					HashType:          txscript.SigHashDefault,
					PrevOutputFetcher: prevOutFetcher,
					ControlBlock:      controlBlockBytes,
					InputIndex:        inputIndex,
					SignMethod:        signMethod,
					TapTweak:          tree.TapscriptRoot,
				}
			}

			signDescs = append(signDescs, signDesc)
		}
	}

	if len(targets) == 0 || totalOutputValue < sweepDustLimit {
		return fmt.Errorf("found %d sweep targets with total value "+
			"of %d satoshis which is below the dust limit of %d",
			len(targets), totalOutputValue, sweepDustLimit)
	}

	// Calculate the fee based on the given fee rate and our weight
	// estimation.
	feeRateKWeight := chainfee.SatPerKVByte(1000 * feeRate).FeePerKWeight()
	totalFee := feeRateKWeight.FeeForWeight(estimator.Weight())

	log.Infof("Fee %d sats of %d total amount (estimated weight %d)",
		totalFee, totalOutputValue, estimator.Weight())

	sweepTx.TxOut = []*wire.TxOut{{
		Value:    int64(totalOutputValue) - int64(totalFee),
		PkScript: sweepScript,
	}}

	// Sign the transaction now.
	var sigHashes = txscript.NewTxSigHashes(sweepTx, prevOutFetcher)
	for idx, desc := range signDescs {
		desc.SigHashes = sigHashes
		desc.InputIndex = idx

		switch {
		// Simple Taproot Channels.
		case desc.SignMethod == input.TaprootScriptSpendSignMethod:
			witness, err := input.TaprootCommitSpendSuccess(
				signer, desc, sweepTx, nil,
			)
			if err != nil {
				return err
			}
			sweepTx.TxIn[idx].Witness = witness

		// Anchor Channels.
		case len(desc.WitnessScript) > 0:
			witness, err := input.CommitSpendToRemoteConfirmed(
				signer, desc, sweepTx,
			)
			if err != nil {
				return err
			}
			sweepTx.TxIn[idx].Witness = witness

		// Static Remote Key Channels.
		default:
			// The txscript library expects the witness script of a
			// P2WKH descriptor to be set to the pkScript of the
			// output...
			desc.WitnessScript = desc.Output.PkScript

			// For CLN we need to activate a flag to make sure we
			// put the correct public key on the witness stack.
			if clnSigner, ok := signer.(*cln.Signer); ok {
				clnSigner.SwapDescKeyAfterDerive = true
			}

			witness, err := input.CommitSpendNoDelay(
				signer, desc, sweepTx,
				len(desc.SingleTweak) == 0,
			)
			if err != nil {
				return err
			}
			sweepTx.TxIn[idx].Witness = witness
		}
	}

	var buf bytes.Buffer
	err := sweepTx.Serialize(&buf)
	if err != nil {
		return err
	}

	// Publish TX.
	if publish {
		response, err := api.PublishTx(
			hex.EncodeToString(buf.Bytes()),
		)
		if err != nil {
			return err
		}
		log.Infof("Published TX %s, response: %s",
			sweepTx.TxHash().String(), response)
	}

	log.Infof("Transaction: %x", buf.Bytes())
	return nil
}

func queryAddressBalances(pubKey *btcec.PublicKey,
	keyDesc *keychain.KeyDescriptor, api *btc.ExplorerAPI,
	knownOutputs []string) ([]*targetAddr, error) {

	var targets []*targetAddr
	queryAddr := func(address btcutil.Address, script []byte,
		scriptTree *input.CommitScriptTree) error {

		if len(knownOutputs) > 0 {
			if !slices.Contains(knownOutputs, address.String()) {
				return nil
			}
		}

		unspent, err := api.Unspent(address.EncodeAddress())
		if err != nil {
			return fmt.Errorf("could not query unspent: %w", err)
		}

		if len(unspent) > 0 {
			log.Infof("Found %d unspent outputs for address %v",
				len(unspent), address.EncodeAddress())
			targets = append(targets, &targetAddr{
				addr:       address,
				keyDesc:    keyDesc,
				vouts:      unspent,
				script:     script,
				scriptTree: scriptTree,
			})
		}

		return nil
	}

	p2wkh, err := lnd.P2WKHAddr(pubKey, chainParams)
	if err != nil {
		return nil, err
	}
	if err := queryAddr(p2wkh, nil, nil); err != nil {
		return nil, err
	}

	p2anchor, script, err := lnd.P2AnchorStaticRemote(pubKey, chainParams)
	if err != nil {
		return nil, err
	}
	if err := queryAddr(p2anchor, script, nil); err != nil {
		return nil, err
	}

	p2tr, scriptTree, err := lnd.P2TaprootStaticRemote(pubKey, chainParams)
	if err != nil {
		return nil, err
	}
	if err := queryAddr(p2tr, nil, scriptTree); err != nil {
		return nil, err
	}

	return targets, nil
}

type ancientChannel struct {
	OP   string `json:"close_outpoint"`
	Addr string `json:"close_addr"`
	CP   string `json:"commit_point"`
	Node string `json:"node"`
}

func findAncientChannels(channels []ancientChannel, numKeys uint32,
	key *hdkeychain.ExtendedKey) ([]ancientChannel, error) {

	if err := fillCache(numKeys, key); err != nil {
		return nil, err
	}

	var foundChannels []ancientChannel
	for _, channel := range channels {
		// Decode the commit point.
		commitPointBytes, err := hex.DecodeString(channel.CP)
		if err != nil {
			return nil, fmt.Errorf("unable to decode commit "+
				"point: %w", err)
		}
		commitPoint, err := btcec.ParsePubKey(commitPointBytes)
		if err != nil {
			return nil, fmt.Errorf("unable to parse commit "+
				"point: %w", err)
		}

		// Create the address for the commit key. The addresses in the
		// file are always for mainnet.
		targetPubKeyHash, _, err := lnd.DecodeAddressHash(
			channel.Addr, &chaincfg.MainNetParams,
		)
		if err != nil {
			return nil, fmt.Errorf("error parsing addr: %w", err)
		}

		_, _, err = keyInCache(numKeys, targetPubKeyHash, commitPoint)
		switch {
		case err == nil:
			foundChannels = append(foundChannels, channel)

		case errors.Is(err, errAddrNotFound):
			// Try next address.

		default:
			return nil, err
		}
	}

	return foundChannels, nil
}

func checkAncientChannelPoints(api *btc.ExplorerAPI, numKeys uint32,
	key *hdkeychain.ExtendedKey) ([]*targetAddr, error) {

	var channels []ancientChannel
	err := json.Unmarshal(ancientChannelPoints, &channels)
	if err != nil {
		return nil, err
	}

	ancientChannels, err := findAncientChannels(channels, numKeys, key)
	if err != nil {
		return nil, err
	}

	targets := make([]*targetAddr, 0, len(ancientChannels))
	for _, ancientChannel := range ancientChannels {
		// Decode the commit point.
		commitPointBytes, err := hex.DecodeString(ancientChannel.CP)
		if err != nil {
			return nil, fmt.Errorf("unable to decode commit "+
				"point: %w", err)
		}
		commitPoint, err := btcec.ParsePubKey(commitPointBytes)
		if err != nil {
			return nil, fmt.Errorf("unable to parse commit point: "+
				"%w", err)
		}

		// Create the address for the commit key. The addresses in the
		// file are always for mainnet.
		targetPubKeyHash, _, err := lnd.DecodeAddressHash(
			ancientChannel.Addr, &chaincfg.MainNetParams,
		)
		if err != nil {
			return nil, fmt.Errorf("error parsing addr: %w", err)
		}
		addr, err := lnd.ParseAddress(
			ancientChannel.Addr, &chaincfg.MainNetParams,
		)
		if err != nil {
			return nil, fmt.Errorf("error parsing addr: %w", err)
		}

		log.Infof("Found private key for address %v in list of "+
			"ancient channels!", addr)

		unspent, err := api.Unspent(addr.EncodeAddress())
		if err != nil {
			return nil, fmt.Errorf("could not query unspent: %w",
				err)
		}

		keyDesc, tweak, err := keyInCache(
			numKeys, targetPubKeyHash, commitPoint,
		)
		if err != nil {
			return nil, err
		}

		targets = append(targets, &targetAddr{
			addr:    addr,
			keyDesc: keyDesc,
			tweak:   tweak,
			vouts:   unspent,
		})
	}

	return targets, nil
}

func listOrFile(listOrPath string) ([]string, error) {
	if lnrpc.FileExists(lncfg.CleanAndExpandPath(listOrPath)) {
		contents, err := os.ReadFile(listOrPath)
		if err != nil {
			return nil, fmt.Errorf("error reading file %s: %w",
				listOrPath, err)
		}

		re := regexp.MustCompile(`[,\s]+`)
		parts := re.Split(string(contents), -1)
		return fn.Filter(parts, func(s string) bool {
			return len(strings.TrimSpace(s)) > 0
		}), nil
	}

	return strings.Split(listOrPath, ","), nil
}
