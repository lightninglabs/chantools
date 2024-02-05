## chantools signmessage

Signs a message with the nodes key, results in the same signature as
`lncli signmessage`

### Synopsis

```
chantools signmessage [flags]
```

### Examples

```
chantools signmessage --msg=foobar
```

### Options

```
      --bip39            read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
  -h, --help             help for signmessage
      --msg string       the message to sign
      --rootkey string   BIP32 HD root key of the wallet to use for decrypting the backup; leave empty to prompt for lnd 24 word aezeed
      --single_hash      single hash the msg instead of double hash (lnd default is false)
```