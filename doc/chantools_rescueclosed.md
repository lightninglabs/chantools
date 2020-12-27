## chantools rescueclosed

Try finding the private keys for funds that are in outputs of remotely force-closed channels

```
chantools rescueclosed [flags]
```

### Options

```
      --bip39                    read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
      --channeldb string         lnd channel.db file to use for rescuing force-closed channels
      --fromchanneldb string     channel input is in the format of an lnd channel.db file
      --fromsummary string       channel input is in the format of chantool's channel summary; specify '-' to read from stdin
  -h, --help                     help for rescueclosed
      --listchannels string      channel input is in the format of lncli's listchannels format; specify '-' to read from stdin
      --pendingchannels string   channel input is in the format of lncli's pendingchannels format; specify '-' to read from stdin
      --rootkey string           BIP32 HD root key of the wallet to use for decrypting the backup; leave empty to prompt for lnd 24 word aezeed
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

