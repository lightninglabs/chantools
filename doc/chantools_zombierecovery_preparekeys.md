## chantools zombierecovery preparekeys

[1/3] Prepare all public keys for a recovery attempt

### Synopsis

Takes a match file, validates it against the seed and 
then adds the first 2500 multisig pubkeys to it.
This must be run by both parties of a channel for a successful recovery. The
next step (makeoffer) takes two such key enriched files and tries to find the
correct ones for the matched channels.

```
chantools zombierecovery preparekeys [flags]
```

### Examples

```
chantools zombierecovery preparekeys \
	--match_file match-xxxx-xx-xx-<pubkey1>-<pubkey2>.json \
	--payout_addr bc1q...
```

### Options

```
      --bip39                read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
  -h, --help                 help for preparekeys
      --match_file string    the match JSON file that was sent to both nodes by the match maker
      --payout_addr string   the address where this node's rescued funds should be sent to, must be a P2WPKH (native SegWit) address
      --rootkey string       BIP32 HD root key of the wallet to use for deriving the multisig keys; leave empty to prompt for lnd 24 word aezeed
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools zombierecovery](chantools_zombierecovery.md)	 - Try rescuing funds stuck in channels with zombie nodes

