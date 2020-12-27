## chantools genimportscript

Generate a script containing the on-chain keys of an lnd wallet that can be imported into other software like bitcoind

```
chantools genimportscript [flags]
```

### Options

```
      --bip39                   read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
      --derivationpath string   use one specific derivation path; specify the first levels of the derivation path before any internal/external branch; Cannot be used in conjunction with --lndpaths
      --format string           format of the generated import script; currently supported are: bitcoin-importwallet, bitcoin-cli and bitcoin-cli-watchonly (default "bitcoin-importwallet")
  -h, --help                    help for genimportscript
      --lndpaths                use all derivation paths that lnd used; results in a large number of results; cannot be used in conjunction with --derivationpath
      --recoverywindow uint32   number of keys to scan per internal/external branch; output will consist of double this amount of keys (default 2500)
      --rescanfrom uint32       block number to rescan from; will be set automatically from the wallet birthday if the lnd 24 word aezeed is entered (default 500000)
      --rootkey string          BIP32 HD root key of the wallet to use for decrypting the backup; leave empty to prompt for lnd 24 word aezeed
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

