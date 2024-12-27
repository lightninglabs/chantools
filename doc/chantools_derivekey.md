## chantools derivekey

Derive a key with a specific derivation path

### Synopsis

This command derives a single key with the given BIP32
derivation path from the root key and prints it to the console.

```
chantools derivekey [flags]
```

### Examples

```
chantools derivekey --path "m/1017'/0'/5'/0/0'" \
	--neuter

chantools derivekey --identity
```

### Options

```
      --bip39             read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
  -h, --help              help for derivekey
      --identity          derive the lnd identity_pubkey
      --neuter            don't output private key(s), only public key(s)
      --path string       BIP32 derivation path to derive; must start with "m/"
      --rootkey string    BIP32 HD root key of the wallet to use for decrypting the backup; leave empty to prompt for lnd 24 word aezeed
      --walletdb string   read the seed/master root key to use for decrypting the backup from an lnd wallet.db file instead of asking for a seed or providing the --rootkey flag
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -s, --signet    Indicates if the public signet parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

