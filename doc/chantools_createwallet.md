## chantools createwallet

Create a new lnd compatible wallet.db file from an existing seed or by generating a new one

### Synopsis

Creates a new wallet that can be used with lnd or with 
chantools. The wallet can be created from an existing seed or a new one can be
generated (use --generateseed).

```
chantools createwallet [flags]
```

### Examples

```
chantools createwallet \
	--walletdbdir ~/.lnd/data/chain/bitcoin/mainnet
```

### Options

```
      --bip39                read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
      --generateseed         generate a new seed instead of using an existing one
  -h, --help                 help for createwallet
      --rootkey string       BIP32 HD root key of the wallet to use for creating the new wallet; leave empty to prompt for lnd 24 word aezeed
      --walletdb string      read the seed/master root key to use for creating the new wallet from an lnd wallet.db file instead of asking for a seed or providing the --rootkey flag
      --walletdbdir string   the folder to create the new wallet.db file in
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -s, --signet    Indicates if the public signet parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

