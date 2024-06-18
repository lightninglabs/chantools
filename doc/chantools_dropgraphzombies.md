## chantools dropgraphzombies

Remove all channels identified as zombies from the graph to force a re-sync of the graph

### Synopsis

This command removes all channels that were identified as
zombies from the local graph.

This will cause lnd to re-download all those channels from the network and can
be helpful to fix a graph that is out of sync with the network.

CAUTION: Running this command will make it impossible to use the channel DB
with an older version of lnd. Downgrading is not possible and you'll need to
run lnd v0.18.0-beta or later after using this command!'

```
chantools dropgraphzombies [flags]
```

### Examples

```
chantools dropgraphzombies \
	--channeldb ~/.lnd/data/graph/mainnet/channel.db
```

### Options

```
      --channeldb string   lnd channel.db file to drop zombies from
  -h, --help               help for dropgraphzombies
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -s, --signet    Indicates if the public signet parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

