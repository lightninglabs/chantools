## chantools dropchannelgraph

Remove all graph related data from a channel DB

### Synopsis

This command removes all graph data from a channel DB,
forcing the lnd node to do a full graph sync.

Or if a single channel is specified, that channel is purged from the graph
without removing any other data.

CAUTION: Running this command will make it impossible to use the channel DB
with an older version of lnd. Downgrading is not possible and you'll need to
run lnd v0.16.0-beta or later after using this command!'

```
chantools dropchannelgraph [flags]
```

### Examples

```
chantools dropchannelgraph \
	--channeldb ~/.lnd/data/graph/mainnet/channel.db \
	--node_identity_key 03......

chantools dropchannelgraph \
	--channeldb ~/.lnd/data/graph/mainnet/channel.db \
	--single_channel 726607861215512345
	--node_identity_key 03......
```

### Options

```
      --channeldb string           lnd channel.db file to dump channels from
      --fix_only                   fix an already empty graph by re-adding the own node's channels
  -h, --help                       help for dropchannelgraph
      --node_identity_key string   your node's identity public key
      --single_channel uint        the single channel identified by its short channel ID (CID) to remove from the graph
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -s, --signet    Indicates if the public signet parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

