## chantools dropchannelgraph

Remove all graph related data from a channel DB

### Synopsis

This command removes all graph data from a channel DB,
forcing the lnd node to do a full graph sync.

CAUTION: Running this command will make it impossible to use the channel DB
with an older version of lnd. Downgrading is not possible and you'll need to
run lnd v0.12.0-beta or later after using this command!'

```
chantools dropchannelgraph [flags]
```

### Examples

```
chantools dropchannelgraph \
	--channeldb ~/.lnd/data/graph/mainnet/channel.db
```

### Options

```
      --channeldb string   lnd channel.db file to dump channels from
  -h, --help               help for dropchannelgraph
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

