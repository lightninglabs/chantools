## chantools zombierecovery findmatches

[0/3] Match maker only: Find matches between registered nodes

### Synopsis

Match maker only: Runs through all the nodes that have
registered their ID on https://www.node-recovery.com and checks whether there
are any matches of channels between them by looking at the whole channel graph.

This command will be run by guggero and the result will be sent to the
registered nodes.

```
chantools zombierecovery findmatches [flags]
```

### Examples

```
chantools zombierecovery findmatches \
	--registrations data.txt \
	--ambosskey <API key>
```

### Options

```
      --ambossdelay duration   the delay between each query to the Amboss GraphQL API (default 4s)
      --ambosskey string       the API key for the Amboss GraphQL API
      --apiurl string          API URL to use (must be esplora compatible) (default "https://blockstream.info/api")
  -h, --help                   help for findmatches
      --registrations string   the raw data.txt where the registrations are stored in
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools zombierecovery](chantools_zombierecovery.md)	 - Try rescuing funds stuck in channels with zombie nodes

