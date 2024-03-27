## chantools signmessage

Sign a message with the node's private key.

### Synopsis

Sign msg with the resident node's private key.
		Returns the signature as a zbase32 string.

```
chantools signmessage [flags]
```

### Examples

```
chantools signmessage --msg=foobar
```

### Options

```
      --bip39             read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
  -h, --help              help for signmessage
      --msg string        the message to sign
      --rootkey string    BIP32 HD root key of the wallet to use for decrypting the backup; leave empty to prompt for lnd 24 word aezeed
      --walletdb string   read the seed/master root key to use fro decrypting the backup from an lnd wallet.db file instead of asking for a seed or providing the --rootkey flag
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -s, --signet    Indicates if the public signet parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

