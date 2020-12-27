## chantools compactdb

Create a copy of a channel.db file in safe/read-only mode

```
chantools compactdb [flags]
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
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

