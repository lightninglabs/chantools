## chantools forceclose

Force-close the last state that is in the channel.db provided

### Synopsis

If you are certain that a node is offline for good (AFTER
you've tried SCB!) and a channel is still open, you can use this method to
force-close your latest state that you have in your channel.db.

**!!! WARNING !!! DANGER !!! WARNING !!!**

If you do this and the state that you publish is *not* the latest state, then
the remote node *could* punish you by taking the whole channel amount *if* they
come online before you can sweep the funds from the time locked (144 - 2000
blocks) transaction *or* they have a watch tower looking out for them.

**This should absolutely be the last resort and you have been warned!**

```
chantools forceclose [flags]
```

### Examples

```
chantools forceclose \
	--fromsummary results/summary-xxxx-yyyy.json
	--channeldb ~/.lnd/data/graph/mainnet/channel.db \
	--publish
```

### Options

```
      --apiurl string            API URL to use (must be esplora compatible) (default "https://api.node-recovery.com")
      --bip39                    read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
      --channeldb string         lnd channel.db file to use for force-closing channels
      --fromchanneldb string     channel input is in the format of an lnd channel.db file
      --fromsummary string       channel input is in the format of chantool's channel summary; specify '-' to read from stdin
  -h, --help                     help for forceclose
      --listchannels string      channel input is in the format of lncli's listchannels format; specify '-' to read from stdin
      --pendingchannels string   channel input is in the format of lncli's pendingchannels format; specify '-' to read from stdin
      --publish                  publish force-closing TX to the chain API instead of just printing the TX
      --rootkey string           BIP32 HD root key of the wallet to use for decrypting the backup; leave empty to prompt for lnd 24 word aezeed
      --walletdb string          read the seed/master root key to use fro decrypting the backup from an lnd wallet.db file instead of asking for a seed or providing the --rootkey flag
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -s, --signet    Indicates if the public signet parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

