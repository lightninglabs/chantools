package lnd

import (
	"bytes"
	"fmt"

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
		return fmt.Errorf("error extracting channel backup: %v", err)
	}
	multi := &chanbackup.Multi{
		Version:       chanbackup.DefaultMultiVersion,
		StaticBackups: singles,
	}
	var b bytes.Buffer
	err = multi.PackToWriter(&b, ring)
	if err != nil {
		return fmt.Errorf("unable to pack backup: %v", err)
	}
	err = multiFile.UpdateAndSwap(b.Bytes())
	if err != nil {
		return fmt.Errorf("unable to write backup file: %v", err)
	}
	return nil
}
