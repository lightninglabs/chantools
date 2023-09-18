## chantools dumpchannels

Dump all channel information from an lnd channel database

### Synopsis

This command dumps all open and pending channels from the
given lnd channel.db gile in a human readable format.

```
chantools dumpchannels [flags]
```

### Examples

```
chantools dumpchannels \
	--channeldb ~/.lnd/data/graph/mainnet/channel.db
```

### Options

```
      --channeldb string   lnd channel.db file to dump channels from
      --closed             dump closed channels instead of open
  -h, --help               help for dumpchannels
      --pending            dump pending channels instead of open
      --waiting_close      dump waiting close channels instead of open
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -s, --signet    Indicates if the public signet parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

