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
	--channelpoint xxxxxxx:xx \
	--sweepaddr bc1qxxxxxxxxx \
	--feerate 10
```

### Options

```
      --bip39                          read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
      --channeldb string               lnd channel.db file to rescue a channel from; must contain the pending channel specified with --channelpoint
      --channelpoint string            funding transaction outpoint of the channel to rescue (<txid>:<txindex>) as it is recorded in the DB
      --confirmedchannelpoint string   channel outpoint that got confirmed on chain (<txid>:<txindex>); normally this is the same as the --channelpoint so it will be set to that value ifthis is left empty
      --feerate uint16                 fee rate to use for the sweep transaction in sat/vByte (default 30)
  -h, --help                           help for rescuefunding
      --rootkey string                 BIP32 HD root key of the wallet to use for deriving keys; leave empty to prompt for lnd 24 word aezeed
      --sweepaddr string               address to sweep the funds to
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

