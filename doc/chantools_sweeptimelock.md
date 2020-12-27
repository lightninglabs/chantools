## chantools sweeptimelock

Sweep the force-closed state after the time lock has expired

```
chantools sweeptimelock [flags]
```

### Options

```
      --apiurl string            API URL to use (must be esplora compatible) (default "https://blockstream.info/api")
      --bip39                    read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
      --feerate uint16           fee rate to use for the sweep transaction in sat/vByte (default 2)
      --fromchanneldb string     channel input is in the format of an lnd channel.db file
      --fromsummary string       channel input is in the format of chantool's channel summary; specify '-' to read from stdin
  -h, --help                     help for sweeptimelock
      --listchannels string      channel input is in the format of lncli's listchannels format; specify '-' to read from stdin
      --maxcsvlimit uint16       maximum CSV limit to use (default 2016)
      --pendingchannels string   channel input is in the format of lncli's pendingchannels format; specify '-' to read from stdin
      --publish                  publish sweep TX to the chain API instead of just printing the TX
      --rootkey string           BIP32 HD root key of the wallet to use for deriving keys; leave empty to prompt for lnd 24 word aezeed
      --sweepaddr string         address to sweep the funds to
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

