package main

import (
	"bytes"
	"encoding/hex"
	"errors"
	"fmt"

	"github.com/btcsuite/btcd/btcec/v2"
	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg/chainhash"
	"github.com/btcsuite/btcd/txscript"
	"github.com/btcsuite/btcd/wire"
	"github.com/lightninglabs/chantools/lnd"
	"github.com/lightningnetwork/lnd/input"
	"github.com/lightningnetwork/lnd/keychain"
	"github.com/lightningnetwork/lnd/lnwallet/chainfee"
	"github.com/lightningnetwork/lnd/shachain"
	"github.com/spf13/cobra"
)

const (
	keyBasePath = "m/1017'/%d'"
	maxKeys     = 500
	maxPoints   = 1000
)

type sweepTimeLockManualCommand struct {
	APIURL                    string
	Publish                   bool
	SweepAddr                 string
	MaxCsvLimit               uint16
	FeeRate                   uint32
	TimeLockAddr              string
	RemoteRevocationBasePoint string

	MaxNumChannelsTotal uint16
	MaxNumChanUpdates   uint64

	ChannelBackup string
	ChannelPoint  string

	rootKey *rootKey
	inputs  *inputFlags
	cmd     *cobra.Command
}

func newSweepTimeLockManualCommand() *cobra.Command {
	cc := &sweepTimeLockManualCommand{}
	cc.cmd = &cobra.Command{
		Use: "sweeptimelockmanual",
		Short: "Sweep the force-closed state of a single channel " +
			"manually if only a channel backup file is available",
		Long: `Sweep the locally force closed state of a single channel
manually if only a channel backup file is available. This can only be used if a
channel is force closed from the local node but then that node's state is lost
and only the channel.backup file is available.

To get the value for --remoterevbasepoint you must use the dumpbackup command,
then look up the value for RemoteChanCfg -> RevocationBasePoint -> PubKey.

Alternatively you can directly use the --frombackup and --channelpoint flags to
pull the required information from the given channel.backup file automatically.

To get the value for --timelockaddr you must look up the channel's funding
output on chain, then follow it to the force close output. The time locked
address is always the one that's longer (because it's P2WSH and not P2PKH).`,
		Example: `chantools sweeptimelockmanual \
	--sweepaddr bc1q..... \
	--timelockaddr bc1q............ \
	--remoterevbasepoint 03xxxxxxx \
	--feerate 10 \
	--publish

chantools sweeptimelockmanual \
	--sweepaddr bc1q..... \
	--timelockaddr bc1q............ \
	--frombackup channel.backup \
	--channelpoint f39310xxxxxxxxxx:1 \
	--feerate 10 \
	--publish`,
		RunE: cc.Execute,
	}
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
	cc.cmd.Flags().Uint16Var(
		&cc.MaxCsvLimit, "maxcsvlimit", defaultCsvLimit, "maximum CSV "+
			"limit to use",
	)
	cc.cmd.Flags().Uint16Var(
		&cc.MaxNumChannelsTotal, "maxnumchanstotal", maxKeys, "maximum "+
			"number of keys to try, set to maximum number of "+
			"channels the local node potentially has or had",
	)
	cc.cmd.Flags().Uint64Var(
		&cc.MaxNumChanUpdates, "maxnumchanupdates", maxPoints,
		"maximum number of channel updates to try, set to maximum "+
			"number of times the channel was used",
	)
	cc.cmd.Flags().Uint32Var(
		&cc.FeeRate, "feerate", defaultFeeSatPerVByte, "fee rate to "+
			"use for the sweep transaction in sat/vByte",
	)
	cc.cmd.Flags().StringVar(
		&cc.TimeLockAddr, "timelockaddr", "", "address of the time "+
			"locked commitment output where the funds are stuck in",
	)
	cc.cmd.Flags().StringVar(
		&cc.RemoteRevocationBasePoint, "remoterevbasepoint", "", ""+
			"remote node's revocation base point, can be found "+
			"in a channel.backup file",
	)
	cc.cmd.Flags().StringVar(
		&cc.ChannelBackup, "frombackup", "", "channel backup file to "+
			"read the channel information from",
	)
	cc.cmd.Flags().StringVar(
		&cc.ChannelPoint, "channelpoint", "", "channel point to use "+
			"for locating the channel in the channel backup file "+
			"specified in the --frombackup flag, "+
			"format: txid:index",
	)

	cc.rootKey = newRootKey(cc.cmd, "deriving keys")
	cc.inputs = newInputFlags(cc.cmd)

	return cc.cmd
}

func (c *sweepTimeLockManualCommand) Execute(_ *cobra.Command, _ []string) error {
	extendedKey, err := c.rootKey.read()
	if err != nil {
		return fmt.Errorf("error reading root key: %w", err)
	}

	// Make sure the sweep and time lock addrs are set.
	err = lnd.CheckAddress(
		c.SweepAddr, chainParams, true, "sweep", lnd.AddrTypeP2WKH,
		lnd.AddrTypeP2TR,
	)
	if err != nil {
		return err
	}

	err = lnd.CheckAddress(
		c.TimeLockAddr, chainParams, true, "time lock",
		lnd.AddrTypeP2WSH,
	)
	if err != nil {
		return err
	}

	var (
		startCsvLimit             uint16
		maxCsvLimit               = c.MaxCsvLimit
		startNumChannelsTotal     uint16
		maxNumChannelsTotal       = c.MaxNumChannelsTotal
		remoteRevocationBasePoint = c.RemoteRevocationBasePoint
	)

	// We either support specifying the remote revocation base point
	// manually, in which case the CSV limit and number of channels are not
	// known, or we can use the channel backup file to get the required
	// information from there directly.
	switch {
	case c.RemoteRevocationBasePoint != "":
		// Nothing to do here but continue below with the info provided
		// by the user.

	case c.ChannelBackup != "":
		if c.ChannelPoint == "" {
			return errors.New("channel point is required with " +
				"--frombackup")
		}

		backupChan, err := lnd.ExtractChannel(
			extendedKey, chainParams, c.ChannelBackup,
			c.ChannelPoint,
		)
		if err != nil {
			return fmt.Errorf("error extracting channel: %w", err)
		}

		remoteCfg := backupChan.RemoteChanCfg
		remoteRevocationBasePoint = remoteCfg.RevocationBasePoint.PubKey

		startCsvLimit = remoteCfg.CsvDelay
		maxCsvLimit = startCsvLimit + 1

		delayPath, err := lnd.ParsePath(
			backupChan.LocalChanCfg.DelayBasePoint.Path,
		)
		if err != nil {
			return fmt.Errorf("error parsing delay path: %w", err)
		}
		if len(delayPath) != 5 {
			return fmt.Errorf("invalid delay path '%v'", delayPath)
		}

		startNumChannelsTotal = uint16(delayPath[4])
		maxNumChannelsTotal = startNumChannelsTotal + 1

	case c.ChannelBackup != "" && c.RemoteRevocationBasePoint != "":
		return errors.New("cannot use both --frombackup and " +
			"--remoterevbasepoint at the same time")

	default:
		return errors.New("either --frombackup or " +
			"--remoterevbasepoint is required")
	}

	// The remote revocation base point must also be set and a valid EC
	// point.
	remoteRevPoint, err := pubKeyFromHex(remoteRevocationBasePoint)
	if err != nil {
		return fmt.Errorf("invalid remote revocation base point: %w",
			err)
	}

	return sweepTimeLockManual(
		extendedKey, c.APIURL, c.SweepAddr, c.TimeLockAddr,
		remoteRevPoint, startCsvLimit, maxCsvLimit,
		startNumChannelsTotal, maxNumChannelsTotal,
		c.MaxNumChanUpdates, c.Publish, c.FeeRate,
	)
}

func sweepTimeLockManual(extendedKey *hdkeychain.ExtendedKey, apiURL string,
	sweepAddr, timeLockAddr string, remoteRevPoint *btcec.PublicKey,
	startCsvTimeout, maxCsvTimeout, startNumChannels, maxNumChannels uint16,
	maxNumChanUpdates uint64, publish bool, feeRate uint32) error {

	log.Debugf("Starting to brute force the time lock script, using: "+
		"remote_rev_base_point=%x, start_csv_limit=%d, "+
		"max_csv_limit=%d, start_num_channels=%d, "+
		"max_num_channels=%d, max_num_chan_updates=%d",
		remoteRevPoint.SerializeCompressed(), startCsvTimeout,
		maxCsvTimeout, startNumChannels, maxNumChannels,
		maxNumChanUpdates)

	// Create signer and transaction template.
	var (
		estimator input.TxWeightEstimator
		signer    = &lnd.Signer{
			ExtendedKey: extendedKey,
			ChainParams: chainParams,
		}
		api = newExplorerAPI(apiURL)
	)

	// First of all, we need to parse the lock addr and make sure we can
	// brute force the script with the information we have. If not, we can't
	// continue anyway.
	lockScript, err := lnd.PrepareWalletAddress(
		timeLockAddr, chainParams, nil, extendedKey, "time lock",
	)
	if err != nil {
		return err
	}
	sweepScript, err := lnd.PrepareWalletAddress(
		sweepAddr, chainParams, &estimator, extendedKey, "sweep",
	)
	if err != nil {
		return err
	}

	// We need to go through a lot of our keys so it makes sense to
	// pre-derive the static part of our key path.
	basePath, err := lnd.ParsePath(fmt.Sprintf(
		keyBasePath, chainParams.HDCoinType,
	))
	if err != nil {
		return fmt.Errorf("could not derive base path: %w", err)
	}
	baseKey, err := lnd.DeriveChildren(extendedKey, basePath)
	if err != nil {
		return fmt.Errorf("could not derive base key: %w", err)
	}

	// Go through all our keys now and try to find the ones that can derive
	// the script. This loop can take very long as it'll nest three times,
	// once for the key index, once for the commit points and once for the
	// CSV values. Most of the calculations should be rather cheap but the
	// number of iterations can go up to maxKeys*maxPoints*maxCsvTimeout.
	var (
		csvTimeout  int32
		script      []byte
		scriptHash  []byte
		delayDesc   *keychain.KeyDescriptor
		commitPoint *btcec.PublicKey
	)
	for i := startNumChannels; i < maxNumChannels; i++ {
		csvTimeout, script, scriptHash, commitPoint, delayDesc, err = tryKey(
			baseKey, remoteRevPoint, startCsvTimeout, maxCsvTimeout,
			lockScript, uint32(i), maxNumChanUpdates,
		)

		if err == nil {
			log.Infof("Found keys at index %d with CSV timeout %d",
				i, csvTimeout)

			break
		}

		log.Infof("Tried %d of %d keys.", i+1, maxKeys)
	}

	// Did we find what we looked for or did we just exhaust all
	// possibilities?
	if script == nil || delayDesc == nil {
		return errors.New("target script not derived")
	}

	// We now know everything we need to construct the sweep transaction,
	// except for what outpoint to sweep. We'll ask the chain API to give
	// us this information.
	tx, txindex, err := api.Outpoint(timeLockAddr)
	if err != nil {
		return fmt.Errorf("error looking up lock address %s on chain: "+
			"%v", timeLockAddr, err)
	}

	sweepTx := wire.NewMsgTx(2)
	sweepValue := int64(tx.Vout[txindex].Value)

	// Create the transaction input.
	txHash, err := chainhash.NewHashFromStr(tx.TXID)
	if err != nil {
		return fmt.Errorf("error parsing tx hash: %w", err)
	}
	sweepTx.TxIn = []*wire.TxIn{{
		PreviousOutPoint: wire.OutPoint{
			Hash:  *txHash,
			Index: uint32(txindex),
		},
		Sequence: input.LockTimeToSequence(
			false, uint32(csvTimeout),
		),
	}}

	// Calculate the fee based on the given fee rate and our weight
	// estimation.
	estimator.AddWitnessInput(input.ToLocalTimeoutWitnessSize)
	feeRateKWeight := chainfee.SatPerKVByte(1000 * feeRate).FeePerKWeight()
	totalFee := feeRateKWeight.FeeForWeight(estimator.Weight())

	// Add our sweep destination output.
	sweepTx.TxOut = []*wire.TxOut{{
		Value:    sweepValue - int64(totalFee),
		PkScript: sweepScript,
	}}

	log.Infof("Fee %d sats of %d total amount (estimated weight %d)",
		totalFee, sweepValue, estimator.Weight())

	// Create the sign descriptor for the input then sign the transaction.
	prevOutFetcher := txscript.NewCannedPrevOutputFetcher(
		scriptHash, sweepValue,
	)
	sigHashes := txscript.NewTxSigHashes(sweepTx, prevOutFetcher)
	signDesc := &input.SignDescriptor{
		KeyDesc: *delayDesc,
		SingleTweak: input.SingleTweakBytes(
			commitPoint, delayDesc.PubKey,
		),
		WitnessScript: script,
		Output: &wire.TxOut{
			PkScript: scriptHash,
			Value:    sweepValue,
		},
		InputIndex:        0,
		SigHashes:         sigHashes,
		PrevOutputFetcher: prevOutFetcher,
		HashType:          txscript.SigHashAll,
	}
	witness, err := input.CommitSpendTimeout(signer, signDesc, sweepTx)
	if err != nil {
		return err
	}
	sweepTx.TxIn[0].Witness = witness

	var buf bytes.Buffer
	err = sweepTx.Serialize(&buf)
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

func tryKey(baseKey *hdkeychain.ExtendedKey, remoteRevPoint *btcec.PublicKey,
	startCsvTimeout, maxCsvTimeout uint16, lockScript []byte, idx uint32,
	maxNumChanUpdates uint64) (int32, []byte, []byte, *btcec.PublicKey,
	*keychain.KeyDescriptor, error) {

	// The easy part first, let's derive the delay base point.
	delayPath := []uint32{
		lnd.HardenedKey(uint32(keychain.KeyFamilyDelayBase)),
		0, idx,
	}
	delayPrivKey, err := lnd.PrivKeyFromPath(baseKey, delayPath)
	if err != nil {
		return 0, nil, nil, nil, nil, err
	}

	// Get the revocation base point first, so we can calculate our
	// commit point. We start with the old way where the revocation index
	// was the same as the other indices. This applies to all channels
	// opened with versions prior to and including lnd v0.12.0-beta.
	revPath := []uint32{
		lnd.HardenedKey(uint32(
			keychain.KeyFamilyRevocationRoot,
		)), 0, idx,
	}
	revRoot, err := lnd.ShaChainFromPath(baseKey, revPath, nil)
	if err != nil {
		return 0, nil, nil, nil, nil, err
	}

	// We now have everything to brute force the lock script. This
	// will take a long while as we both have to go through commit
	// points and CSV values.
	csvTimeout, script, scriptHash, commitPoint, err := bruteForceDelayPoint(
		delayPrivKey.PubKey(), remoteRevPoint, revRoot, lockScript,
		startCsvTimeout, maxCsvTimeout, maxNumChanUpdates,
	)
	if err == nil {
		return csvTimeout, script, scriptHash, commitPoint,
			&keychain.KeyDescriptor{
				PubKey: delayPrivKey.PubKey(),
				KeyLocator: keychain.KeyLocator{
					Family: keychain.KeyFamilyDelayBase,
					Index:  idx,
				},
			}, nil
	}

	// We could not derive the secrets to sweep the to_local output using
	// the old shachain root creation. Starting with lnd release
	// v0.13.0-beta the index for the revocation path creating the shachain
	// root changed. Now the shachain root is created using ECDH
	// with the local multisig public key
	// (for mainnet: m/1017'/0'/1'/0/idx). But we need to account for a
	// special case here. If the node was started with a version prior to
	// and including v0.12.0-beta the idx for the new shachain root
	// revocation is not one larger because idx 0 was already used for the
	// old creation scheme hence we need to replicate this behaviour here.
	// First trying the shachain root creation with the same index and if
	// this does not derive the secrets we increase the index of the
	// revocation key path by one (for mainnet: m/1017'/0'/5'/0/idx+1).
	// The exact path which was used for the shachain root can be seen
	// in the channel.backup file for every specific channel. The old
	// scheme has always a public key specified.The new one uses a key
	// locator and does not have a public key specified (nil).
	// Example
	//     ShaChainRootDesc: (dump.KeyDescriptor) {
	// 	Path: (string) (len=17) "m/1017'/1'/5'/0/1",
	// 	PubKey: (string) (len=5) "<nil>"
	//
	// For more details:
	// https://github.com/lightningnetwork/lnd/commit/bb84f0ebc88620050dec7cf4be6283f5cba8b920
	//
	// Now the new shachain root revocation scheme is tried with
	// two different indicies as described above.
	revPath2 := []uint32{
		lnd.HardenedKey(uint32(
			keychain.KeyFamilyRevocationRoot,
		)), 0, idx,
	}

	// Now we try the same with the new revocation producer format.
	multiSigPath := []uint32{
		lnd.HardenedKey(uint32(keychain.KeyFamilyMultiSig)),
		0, idx,
	}
	multiSigPrivKey, err := lnd.PrivKeyFromPath(baseKey, multiSigPath)
	if err != nil {
		return 0, nil, nil, nil, nil, err
	}

	revRoot2, err := lnd.ShaChainFromPath(
		baseKey, revPath2, multiSigPrivKey.PubKey(),
	)
	if err != nil {
		return 0, nil, nil, nil, nil, err
	}

	csvTimeout, script, scriptHash, commitPoint, err = bruteForceDelayPoint(
		delayPrivKey.PubKey(), remoteRevPoint, revRoot2, lockScript,
		startCsvTimeout, maxCsvTimeout, maxNumChanUpdates,
	)
	if err == nil {
		return csvTimeout, script, scriptHash, commitPoint,
			&keychain.KeyDescriptor{
				PubKey: delayPrivKey.PubKey(),
				KeyLocator: keychain.KeyLocator{
					Family: keychain.KeyFamilyDelayBase,
					Index:  idx,
				},
			}, nil
	}

	// Now we try to increase the index by 1 to account for the situation
	// where the node was started with a version after (including)
	// v0.13.0-beta
	revPath3 := []uint32{
		lnd.HardenedKey(uint32(
			keychain.KeyFamilyRevocationRoot,
		)), 0, idx + 1,
	}

	// Now we try the same with the new revocation producer format.
	revRoot3, err := lnd.ShaChainFromPath(
		baseKey, revPath3, multiSigPrivKey.PubKey(),
	)
	if err != nil {
		return 0, nil, nil, nil, nil, err
	}

	csvTimeout, script, scriptHash, commitPoint, err = bruteForceDelayPoint(
		delayPrivKey.PubKey(), remoteRevPoint, revRoot3, lockScript,
		startCsvTimeout, maxCsvTimeout, maxNumChanUpdates,
	)
	if err == nil {
		return csvTimeout, script, scriptHash, commitPoint,
			&keychain.KeyDescriptor{
				PubKey: delayPrivKey.PubKey(),
				KeyLocator: keychain.KeyLocator{
					Family: keychain.KeyFamilyDelayBase,
					Index:  idx,
				},
			}, nil
	}

	return 0, nil, nil, nil, nil, errors.New("target script not derived")
}

func bruteForceDelayPoint(delayBase, revBase *btcec.PublicKey,
	revRoot *shachain.RevocationProducer, lockScript []byte,
	startCsvTimeout, maxCsvTimeout uint16, maxChanUpdates uint64) (int32,
	[]byte, []byte, *btcec.PublicKey, error) {

	for i := range maxChanUpdates {
		revPreimage, err := revRoot.AtIndex(i)
		if err != nil {
			return 0, nil, nil, nil, err
		}
		commitPoint := input.ComputeCommitmentPoint(revPreimage[:])

		csvTimeout, script, scriptHash, err := bruteForceDelay(
			input.TweakPubKey(delayBase, commitPoint),
			input.DeriveRevocationPubkey(revBase, commitPoint),
			lockScript, startCsvTimeout, maxCsvTimeout,
		)

		if err != nil {
			continue
		}

		return csvTimeout, script, scriptHash, commitPoint, nil
	}

	return 0, nil, nil, nil, errors.New("target script not derived")
}
