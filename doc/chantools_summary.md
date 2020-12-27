## chantools summary

Compile a summary about the current state of channels

### Synopsis

From a list of channels, find out what their state is by
querying the funding transaction on a block explorer API.

```
chantools summary [flags]
```

### Examples

```
lncli listchannels | chantools summary --listchannels -

chantools summary --fromchanneldb ~/.lnd/data/graph/mainnet/channel.db
```

### Options

```
      --apiurl string            API URL to use (must be esplora compatible) (default "https://blockstream.info/api")
      --fromchanneldb string     channel input is in the format of an lnd channel.db file
      --fromsummary string       channel input is in the format of chantool's channel summary; specify '-' to read from stdin
  -h, --help                     help for summary
      --listchannels string      channel input is in the format of lncli's listchannels format; specify '-' to read from stdin
      --pendingchannels string   channel input is in the format of lncli's pendingchannels format; specify '-' to read from stdin
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

