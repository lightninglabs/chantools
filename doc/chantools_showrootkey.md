## chantools showrootkey

Extract and show the BIP32 HD root key from the 24 word lnd aezeed

### Synopsis

This command converts the 24 word lnd aezeed phrase and
password to the BIP32 HD root key that is used as the --rootkey flag in other
commands of this tool.

```
chantools showrootkey [flags]
```

### Examples

```
chantools showrootkey
```

### Options

```
      --bip39             read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
  -h, --help              help for showrootkey
      --rootkey string    BIP32 HD root key of the wallet to use for decrypting the backup; leave empty to prompt for lnd 24 word aezeed
      --walletdb string   read the seed/master root key to use for decrypting the backup from an lnd wallet.db file instead of asking for a seed or providing the --rootkey flag
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

