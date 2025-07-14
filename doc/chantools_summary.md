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
      --ancient                  Create summary of ancient channel closes with un-swept outputs
      --ancientstats string      Create summary of ancient channel closes with un-swept outputs and print stats for the given list of channels
      --apiurl string            API URL to use (must be esplora compatible) (default "https://api.node-recovery.com")
      --fromchanneldb string     channel input is in the format of an lnd channel.db file
      --fromchanneldump string   channel input is in the format of a channel dump file
      --fromsummary string       channel input is in the format of chantool's channel summary; specify '-' to read from stdin
  -h, --help                     help for summary
      --listchannels string      channel input is in the format of lncli's listchannels format; specify '-' to read from stdin
      --pendingchannels string   channel input is in the format of lncli's pendingchannels format; specify '-' to read from stdin
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

