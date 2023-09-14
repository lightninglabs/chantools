## chantools triggerforceclose

Connect to a CLN peer and send a custom message to trigger a force close of the specified channel

### Synopsis

Certain versions of CLN didn't properly react to error
messages sent by peers and therefore didn't follow the DLP protocol to recover
channel funds using SCB. This command can be used to trigger a force close with
those earlier versions of CLN (this command will not work for lnd peers or CLN
peers of a different version).

```
chantools triggerforceclose [flags]
```

### Examples

```
chantools triggerforceclose \
	--peer 03abce...@xx.yy.zz.aa:9735 \
	--channel_point abcdef01234...:x
```

### Options

```
      --apiurl string          API URL to use (must be esplora compatible) (default "https://blockstream.info/api")
      --bip39                  read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
      --channel_point string   funding transaction outpoint of the channel to trigger the force close of (<txid>:<txindex>)
  -h, --help                   help for triggerforceclose
      --peer string            remote peer address (<pubkey>@<host>[:<port>])
      --rootkey string         BIP32 HD root key of the wallet to use for deriving the identity key; leave empty to prompt for lnd 24 word aezeed
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

