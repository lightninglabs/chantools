## chantools pullanchor

Attempt to CPFP an anchor output of a channel

### Synopsis

Use this command to confirm a channel force close
transaction of an anchor output channel type. This will attempt to CPFP the
330 byte anchor output created for your node.

```
chantools pullanchor [flags]
```

### Examples

```
chantools pullanchor \
	--sponsorinput txid:vout \
	--anchoraddr bc1q..... \
	--changeaddr bc1q..... \
	--feerate 30
```

### Options

```
      --anchoraddr stringArray   the address of the anchor output (p2wsh or p2tr output with 330 satoshis) that should be pulled; can be specified multiple times per command to pull multiple anchors with a single transaction
      --apiurl string            API URL to use (must be esplora compatible) (default "https://api.node-recovery.com")
      --bip39                    read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
      --changeaddr string        the change address to send the remaining funds back to; specify 'fromseed' to derive a new address from the seed automatically
      --feerate uint32           fee rate to use for the sweep transaction in sat/vByte (default 30)
  -h, --help                     help for pullanchor
      --rootkey string           BIP32 HD root key of the wallet to use for deriving keys; leave empty to prompt for lnd 24 word aezeed
      --sponsorinput string      the input to use to sponsor the CPFP transaction; must be owned by the lnd node that owns the anchor output
      --walletdb string          read the seed/master root key to use for deriving keys from an lnd wallet.db file instead of asking for a seed or providing the --rootkey flag
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

