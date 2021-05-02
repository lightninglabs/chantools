## chantools sweeptimelockmanual

Sweep the force-closed state of a single channel manually if only a channel backup file is available

### Synopsis

Sweep the locally force closed state of a single channel
manually if only a channel backup file is available. This can only be used if a
channel is force closed from the local node but then that node's state is lost
and only the channel.backup file is available.

To get the value for --remoterevbasepoint you must use the dumpbackup command,
then look up the value for RemoteChanCfg -> RevocationBasePoint -> PubKey.

To get the value for --timelockaddr you must look up the channel's funding
output on chain, then follow it to the force close output. The time locked
address is always the one that's longer (because it's P2WSH and not P2PKH).

```
chantools sweeptimelockmanual [flags]
```

### Examples

```
chantools sweeptimelockmanual \
	--sweepaddr bc1q..... \
	--timelockaddr bc1q............ \
	--remoterevbasepoint 03xxxxxxx \
	--feerate 10 \
	--publish
```

### Options

```
      --apiurl string               API URL to use (must be esplora compatible) (default "https://blockstream.info/api")
      --bip39                       read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
      --feerate uint16              fee rate to use for the sweep transaction in sat/vByte (default 2)
      --fromchanneldb string        channel input is in the format of an lnd channel.db file
      --fromsummary string          channel input is in the format of chantool's channel summary; specify '-' to read from stdin
  -h, --help                        help for sweeptimelockmanual
      --listchannels string         channel input is in the format of lncli's listchannels format; specify '-' to read from stdin
      --maxcsvlimit uint16          maximum CSV limit to use (default 2016)
      --pendingchannels string      channel input is in the format of lncli's pendingchannels format; specify '-' to read from stdin
      --publish                     publish sweep TX to the chain API instead of just printing the TX
      --remoterevbasepoint string   remote node's revocation base point, can be found in a channel.backup file
      --rootkey string              BIP32 HD root key of the wallet to use for deriving keys; leave empty to prompt for lnd 24 word aezeed
      --sweepaddr string            address to sweep the funds to
      --timelockaddr string         address of the time locked commitment output where the funds are stuck in
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

