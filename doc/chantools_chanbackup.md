## chantools chanbackup

Create a channel.backup file from a channel database

### Synopsis

This command creates a new channel.backup from a 
channel.db file.

```
chantools chanbackup [flags]
```

### Examples

```
chantools chanbackup \
	--channeldb ~/.lnd/data/graph/mainnet/channel.db \
	--multi_file new_channel_backup.backup
```

### Options

```
      --bip39               read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
      --channeldb string    lnd channel.db file to create the backup from
  -h, --help                help for chanbackup
      --multi_file string   lnd channel.backup file to create
      --rootkey string      BIP32 HD root key of the wallet to use for creating the backup; leave empty to prompt for lnd 24 word aezeed
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -s, --signet    Indicates if the public signet parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

