package lnd

import (
	"bytes"
	"fmt"

	"github.com/btcsuite/btcd/btcutil/hdkeychain"
	"github.com/btcsuite/btcd/chaincfg"
	"github.com/lightninglabs/chantools/dump"
	"github.com/lightningnetwork/lnd/chanbackup"
	"github.com/lightningnetwork/lnd/channeldb"
	"github.com/lightningnetwork/lnd/keychain"
)

// CreateChannelBackup creates a channel backup file from all channels found in
// the given DB file, encrypted with the key in the key ring.
func CreateChannelBackup(db *channeldb.DB, multiFile *chanbackup.MultiFile,
	ring keychain.KeyRing) error {

	singles, err := chanbackup.FetchStaticChanBackups(
		db.ChannelStateDB(), db,
	)
	if err != nil {
		return fmt.Errorf("error extracting channel backup: %w", err)
	}
	multi := &chanbackup.Multi{
		Version:       chanbackup.DefaultMultiVersion,
		StaticBackups: singles,
	}
	var b bytes.Buffer
	err = multi.PackToWriter(&b, ring)
	if err != nil {
		return fmt.Errorf("unable to pack backup: %w", err)
	}
	err = multiFile.UpdateAndSwap(b.Bytes())
	if err != nil {
		return fmt.Errorf("unable to write backup file: %w", err)
	}
	return nil
}

// ExtractChannel extracts a single channel from the given backup file and
// returns it as a dump.BackupSingle struct.
func ExtractChannel(extendedKey *hdkeychain.ExtendedKey,
	chainParams *chaincfg.Params, multiFilePath,
	channelPoint string) (*dump.BackupSingle, error) {

	const noBackupArchive = false
	multiFile := chanbackup.NewMultiFile(multiFilePath, noBackupArchive)
	keyRing := &HDKeyRing{
		ExtendedKey: extendedKey,
		ChainParams: chainParams,
	}

	multi, err := multiFile.ExtractMulti(keyRing)
	if err != nil {
		return nil, fmt.Errorf("could not extract multi file: %w", err)
	}

	channels := dump.BackupDump(multi, chainParams)
	for _, channel := range channels {
		if channel.FundingOutpoint == channelPoint {
			return &channel, nil
		}
	}

	return nil, fmt.Errorf("channel %s not found in backup", channelPoint)
}
