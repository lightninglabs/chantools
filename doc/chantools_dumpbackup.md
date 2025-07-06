## chantools dumpbackup

Dump the content of a channel.backup file

### Synopsis

This command dumps all information that is inside a 
channel.backup file in a human readable format.

```
chantools dumpbackup [flags]
```

### Examples

```
chantools dumpbackup \
	--multi_file ~/.lnd/data/chain/bitcoin/mainnet/channel.backup
```

### Options

```
      --bip39               read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
  -h, --help                help for dumpbackup
      --multi_file string   lnd channel.backup file to dump
      --rootkey string      BIP32 HD root key of the wallet to use for decrypting the backup; leave empty to prompt for lnd 24 word aezeed
      --walletdb string     read the seed/master root key to use for decrypting the backup from an lnd wallet.db file instead of asking for a seed or providing the --rootkey flag
```

### Options inherited from parent commands

```
      --nologfile   If set, no log file will be created. This is useful for testing purposes where we don't want to create a log file.
  -r, --regtest     Indicates if regtest parameters should be used
  -s, --signet      Indicates if the public signet parameters should be used
  -t, --testnet     Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

