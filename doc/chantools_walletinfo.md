## chantools walletinfo

Shows info about an lnd wallet.db file and optionally extracts the BIP32 HD root key

```
chantools walletinfo [flags]
```

### Options

```
  -h, --help              help for walletinfo
      --walletdb string   lnd wallet.db file to dump the contents from
      --withrootkey       print BIP32 HD root key of wallet to standard out
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

