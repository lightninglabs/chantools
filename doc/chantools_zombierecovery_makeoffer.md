## chantools zombierecovery makeoffer

[2/3] Make an offer on how to split the funds to recover

### Synopsis

After both parties have prepared their keys with the
'preparekeys' command and have  exchanged the files generated from that step,
one party has to create an offer on how to split the funds that are in the
channels to be rescued.
If the other party agrees with the offer, they can sign and publish the offer
with the 'signoffer' command. If the other party does not agree, they can create
a counter offer.

```
chantools zombierecovery makeoffer [flags]
```

### Examples

```
chantools zombierecovery makeoffer \
	--node1_keys preparedkeys-xxxx-xx-xx-<pubkey1>.json \
	--node2_keys preparedkeys-xxxx-xx-xx-<pubkey2>.json \
	--feerate 15
```

### Options

```
      --bip39               read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
      --feerate uint32      fee rate to use for the sweep transaction in sat/vByte (default 30)
  -h, --help                help for makeoffer
      --node1_keys string   the JSON file generated in theprevious step ('preparekeys') command of node 1
      --node2_keys string   the JSON file generated in theprevious step ('preparekeys') command of node 2
      --rootkey string      BIP32 HD root key of the wallet to use for signing the offer; leave empty to prompt for lnd 24 word aezeed
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools zombierecovery](chantools_zombierecovery.md)	 - Try rescuing funds stuck in channels with zombie nodes

