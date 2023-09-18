## chantools compactdb

Create a copy of a channel.db file in safe/read-only mode

### Synopsis

This command opens a database in read-only mode and tries
to create a copy of it to a destination file, compacting it in the process.

```
chantools compactdb [flags]
```

### Examples

```
chantools compactdb \
	--sourcedb ~/.lnd/data/graph/mainnet/channel.db \
	--destdb ./results/compacted.db
```

### Options

```
      --destdb string     new lnd channel.db file to copy the compacted database to
  -h, --help              help for compactdb
      --sourcedb string   lnd channel.db file to create the database backup from
      --txmaxsize int     maximum transaction size (default 65536)
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -s, --signet    Indicates if the public signet parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

