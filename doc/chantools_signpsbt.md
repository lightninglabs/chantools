## chantools signpsbt

Sign a Partially Signed Bitcoin Transaction (PSBT)

### Synopsis

Sign a PSBT with a master root key. The PSBT must contain
an input that is owned by the master root key.

```
chantools signpsbt [flags]
```

### Examples

```
chantools signpsbt \
	--psbt <the_base64_encoded_psbt>

chantools signpsbt --fromrawpsbtfile <file_with_psbt>
```

### Options

```
      --bip39                    read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
      --fromrawpsbtfile string   the file containing the raw, binary encoded PSBT packet to sign
  -h, --help                     help for signpsbt
      --psbt string              Partially Signed Bitcoin Transaction to sign
      --rootkey string           BIP32 HD root key of the wallet to use for signing the PSBT; leave empty to prompt for lnd 24 word aezeed
      --torawpsbtfile string     the file to write the resulting signed raw, binary encoded PSBT packet to
      --walletdb string          read the seed/master root key to use for signing the PSBT from an lnd wallet.db file instead of asking for a seed or providing the --rootkey flag
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

