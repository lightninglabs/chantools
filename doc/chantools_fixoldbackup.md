## chantools fixoldbackup

Fixes an old channel.backup file that is affected by the lnd issue #3881 (unable to derive shachain root key)

### Synopsis

Fixes an old channel.backup file that is affected by the
lnd issue [#3881](https://github.com/lightningnetwork/lnd/issues/3881)
(<code>[lncli] unable to restore chan backups: rpc error: code = Unknown desc =
unable to unpack chan backup: unable to derive shachain root key: unable to
derive private key</code>).

```
chantools fixoldbackup [flags]
```

### Examples

```
chantools fixoldbackup \
	--multi_file ~/.lnd/data/chain/bitcoin/mainnet/channel.backup
```

### Options

```
      --bip39               read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
  -h, --help                help for fixoldbackup
      --multi_file string   lnd channel.backup file to fix
      --rootkey string      BIP32 HD root key of the wallet to use for decrypting the backup; leave empty to prompt for lnd 24 word aezeed
      --walletdb string     read the seed/master root key to use fro decrypting the backup from an lnd wallet.db file instead of asking for a seed or providing the --rootkey flag
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -s, --signet    Indicates if the public signet parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

