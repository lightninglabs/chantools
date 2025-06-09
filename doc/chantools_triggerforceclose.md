## chantools triggerforceclose

Connect to a Lightning Network peer and send specific messages to trigger a force close of the specified channel

### Synopsis

Asks the specified remote peer to force close a specific
channel by first sending a channel re-establish message, and if that doesn't
work, a custom error message (in case the peer is a specific version of CLN that
does not properly respond to a Data Loss Protection re-establish message).'

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
      --apiurl string          API URL to use (must be esplora compatible) (default "https://api.node-recovery.com")
      --bip39                  read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
      --channel_point string   funding transaction outpoint of the channel to trigger the force close of (<txid>:<txindex>)
  -h, --help                   help for triggerforceclose
      --hsm_secret string      the hex encoded HSM secret to use for deriving the node key for a CLN node; obtain by running 'xxd -p -c32 ~/.lightning/bitcoin/hsm_secret'
      --peer string            remote peer address (<pubkey>@<host>[:<port>])
      --rootkey string         BIP32 HD root key of the wallet to use for deriving the identity key; leave empty to prompt for lnd 24 word aezeed
      --torproxy string        SOCKS5 proxy to use for Tor connections (to .onion addresses)
      --walletdb string        read the seed/master root key to use for deriving the identity key from an lnd wallet.db file instead of asking for a seed or providing the --rootkey flag
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -s, --signet    Indicates if the public signet parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

