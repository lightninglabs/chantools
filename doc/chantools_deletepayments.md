## chantools deletepayments

Remove all (failed) payments from a channel DB

### Synopsis

This command removes all payments from a channel DB.
If only the failed payments should be deleted (and not the successful ones), the
--failedonly flag can be specified.

CAUTION: Running this command will make it impossible to use the channel DB
with an older version of lnd. Downgrading is not possible and you'll need to
run lnd v0.19.0-beta or later after using this command!'

```
chantools deletepayments [flags]
```

### Examples

```
chantools deletepayments --failedonly \
	--channeldb ~/.lnd/data/graph/mainnet/channel.db
```

### Options

```
      --channeldb string   lnd channel.db file to dump channels from
      --failedonly         don't delete all payments, only failed ones
  -h, --help               help for deletepayments
```

### Options inherited from parent commands

```
      --nologfile           If set, no log file will be created. This is useful for testing purposes where we don't want to create a log file.
  -r, --regtest             Indicates if regtest parameters should be used
      --resultsdir string   Directory where results should be stored (default "./results")
  -s, --signet              Indicates if the public signet parameters should be used
  -t, --testnet             Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

