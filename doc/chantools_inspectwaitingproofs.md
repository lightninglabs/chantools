## chantools inspectwaitingproofs

Inspect lnd waiting proofs for startup-fatal records without modifying the database

### Synopsis

This command opens an offline copy of an lnd channel.db in
strictly read-only mode and inspects the waitingproofs bucket. It identifies
records that the lnd v0.21 typed waiting-proof decoder would reject, including
legacy remote proofs that are misread as V2 MuSig2 nonces.

The report also prints the raw channel DB version key status. A
`db_version_status=db_version_key_missing` result means the `metadata` bucket is
present but `metadata/dbp` is absent. Affected lnd versions can interpret that
missing key as the latest schema version, which can explain why legacy waiting
proofs were left unmigrated.

Always stop lnd and create a copy of channel.db first. This command does not
repair or delete anything. Do not share channel.db because it contains
sensitive channel state.

```
chantools inspectwaitingproofs [flags]
```

### Examples

```
chantools inspectwaitingproofs \
	--channeldb /tmp/channel.db.waitingproof-check
```

### Options

```
      --channeldb string   offline copy of the lnd channel.db file to inspect
  -h, --help               help for inspectwaitingproofs
      --json               print the inspection report as JSON
```

### Options inherited from parent commands

```
      --nologfile           If set, no log file will be created. This is useful for testing purposes where we don't want to create a log file.
  -r, --regtest             Indicates if regtest parameters should be used
      --resultsdir string   Directory where results should be stored (default "./results")
  -s, --signet              Indicates if the public signet parameters should be used
  -t, --testnet             Indicates if testnet parameters should be used
      --testnet4            Indicates if testnet4 parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels
