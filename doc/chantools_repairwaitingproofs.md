## chantools repairwaitingproofs

Repair legacy lnd waiting proofs that can prevent startup

### Synopsis

This command repairs the specific waitingproofs bucket state that
can prevent affected lnd v0.21 nodes from starting with errors such as:

  invalid public key: unsupported format: 3d

The command only migrates clean legacy V1 waiting proof records from the old
format to the typed V1 format expected by lnd v0.21. It does not write or
invent metadata/dbp, and it refuses to overwrite conflicting typed records.

By default this command performs a dry run. To modify the database, stop lnd,
make sure you are operating on the intended channel.db, and pass --commit. When
--commit is used, the command creates a timestamped backup next to channel.db
before writing any changes.

```
chantools repairwaitingproofs [flags]
```

### Examples

```
chantools repairwaitingproofs \
	--channeldb ~/.lnd/data/graph/mainnet/channel.db

chantools repairwaitingproofs \
	--channeldb ~/.lnd/data/graph/mainnet/channel.db \
	--commit
```

### Options

```
      --channeldb string   offline lnd channel.db file to repair
      --commit             modify channel.db after creating a backup; without this flag only a dry run is performed
  -h, --help               help for repairwaitingproofs
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

