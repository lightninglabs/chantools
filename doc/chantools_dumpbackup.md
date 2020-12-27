## chantools dumpbackup

Dump the content of a channel.backup file

```
chantools dumpbackup [flags]
```

### Options

```
      --bip39               read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
  -h, --help                help for dumpbackup
      --multi_file string   lnd channel.backup file to dump
      --rootkey string      BIP32 HD root key of the wallet to use for decrypting the backup; leave empty to prompt for lnd 24 word aezeed
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

