## chantools vanitygen

Generate a seed with a custom lnd node identity public key that starts with the given prefix

```
chantools vanitygen [flags]
```

### Options

```
  -h, --help            help for vanitygen
      --prefix string   hex encoded prefix to find in node public key
      --threads uint8   number of parallel threads (default 4)
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

