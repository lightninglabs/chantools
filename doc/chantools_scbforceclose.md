## chantools scbforceclose

Force-close the last state that is in the SCB provided

### Synopsis


If you are certain that a node is offline for good (AFTER you've tried SCB!)
and a channel is still open, you can use this method to force-close your
latest state that you have in your channel.db.

**!!! WARNING !!! DANGER !!! WARNING !!!**

If you do this and the state that you publish is *not* the latest state, then
the remote node *could* punish you by taking the whole channel amount *if* they
come online before you can sweep the funds from the time locked (144 - 2000
blocks) transaction *or* they have a watch tower looking out for them.

**This should absolutely be the last resort and you have been warned!**

```
chantools scbforceclose [flags]
```

### Examples

```
chantools scbforceclose --multi_file channel.backup
```

### Options

```
      --apiurl string          API URL to use (must be esplora compatible) (default "https://api.node-recovery.com")
      --bip39                  read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
  -h, --help                   help for scbforceclose
      --multi_backup string    a hex encoded multi-channel backup obtained from exportchanbackup for force-closing channels
      --multi_file string      the path to a single-channel backup file (channel.backup)
      --publish                publish force-closing TX to the chain API instead of just printing the TX
      --rootkey string         BIP32 HD root key of the wallet to use for decrypting the backup and signing tx; leave empty to prompt for lnd 24 word aezeed
      --single_backup string   a hex encoded single channel backup obtained from exportchanbackup for force-closing channels
      --single_file string     the path to a single-channel backup file
      --walletdb string        read the seed/master root key to use for decrypting the backup and signing tx from an lnd wallet.db file instead of asking for a seed or providing the --rootkey flag
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

