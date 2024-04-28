## chantools removechannel

Remove a single channel from the given channel DB

### Synopsis

Opens the given channel DB in write mode and removes one
single channel from it. This means giving up on any state (and therefore coins)
of that channel and should only be used if the funding transaction of the
channel was never confirmed on chain!

CAUTION: Running this command will make it impossible to use the channel DB
with an older version of lnd. Downgrading is not possible and you'll need to
run lnd v0.17.4-beta or later after using this command!

```
chantools removechannel [flags]
```

### Examples

```
chantools removechannel \
	--channeldb ~/.lnd/data/graph/mainnet/channel.db \
	--channel 3149764effbe82718b280de425277e5e7b245a4573aa4a0203ac12cee1c37816:0
```

### Options

```
      --channel string     channel to remove from the DB file, identified by its channel point (<txid>:<txindex>)
      --channeldb string   lnd channel.backup file to remove the channel from
  -h, --help               help for removechannel
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -s, --signet    Indicates if the public signet parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

