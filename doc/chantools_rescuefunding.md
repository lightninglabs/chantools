## chantools rescuefunding

Rescue funds locked in a funding multisig output that never resulted in a proper channel; this is the command the initiator of the channel needs to run

### Synopsis

This is part 1 of a two phase process to rescue a channel
funding output that was created on chain by accident but never resulted in a
proper channel and no commitment transactions exist to spend the funds locked in
the 2-of-2 multisig.

**You need the cooperation of the channel partner (remote node) for this to
work**! They need to run the second command of this process: signrescuefunding

If successful, this will create a PSBT that then has to be sent to the channel
partner (remote node operator).

```
chantools rescuefunding [flags]
```

### Examples

```
chantools rescuefunding \
	--channeldb ~/.lnd/data/graph/mainnet/channel.db \
	--dbchannelpoint xxxxxxx:xx \
	--sweepaddr bc1qxxxxxxxxx \
	--feerate 10

chantools rescuefunding \
	--confirmedchannelpoint xxxxxxx:xx \
	--localkeyindex x \
	--remotepubkey 0xxxxxxxxxxxxxxxx \
	--sweepaddr bc1qxxxxxxxxx \
	--feerate 10
```

### Options

```
      --apiurl string                  API URL to use (must be esplora compatible) (default "https://api.node-recovery.com")
      --bip39                          read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
      --channeldb string               lnd channel.db file to rescue a channel from; must contain the pending channel specified with --channelpoint
      --confirmedchannelpoint string   channel outpoint that got confirmed on chain (<txid>:<txindex>); normally this is the same as the --dbchannelpoint so it will be set to that value ifthis is left empty
      --dbchannelpoint string          funding transaction outpoint of the channel to rescue (<txid>:<txindex>) as it is recorded in the DB
      --feerate uint32                 fee rate to use for the sweep transaction in sat/vByte (default 30)
  -h, --help                           help for rescuefunding
      --localkeyindex uint32           in case a channel DB is not available (but perhaps a channel backup file), the derivation index of the local multisig public key can be specified manually
      --remotepubkey string            in case a channel DB is not available (but perhaps a channel backup file), the remote multisig public key can be specified manually
      --rootkey string                 BIP32 HD root key of the wallet to use for deriving keys; leave empty to prompt for lnd 24 word aezeed
      --sweepaddr string               address to recover the funds to; specify 'fromseed' to derive a new address from the seed automatically
      --walletdb string                read the seed/master root key to use for deriving keys from an lnd wallet.db file instead of asking for a seed or providing the --rootkey flag
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

