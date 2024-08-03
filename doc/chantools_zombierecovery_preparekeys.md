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
      --hsm_secret string    the hex encoded HSM secret to use for deriving the multisig keys for a CLN node; obtain by running 'xxd -p -c32 ~/.lightning/bitcoin/hsm_secret'
      --match_file string    the match JSON file that was sent to both nodes by the match maker
      --num_keys uint32      the number of multisig keys to derive (default 2500)
      --payout_addr string   the address where this node's rescued funds should be sent to, must be a P2WPKH (native SegWit) or P2TR (Taproot) address
      --rootkey string       BIP32 HD root key of the wallet to use for deriving the multisig keys; leave empty to prompt for lnd 24 word aezeed
      --walletdb string      read the seed/master root key to use for deriving the multisig keys from an lnd wallet.db file instead of asking for a seed or providing the --rootkey flag
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -s, --signet    Indicates if the public signet parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools zombierecovery](chantools_zombierecovery.md)	 - Try rescuing funds stuck in channels with zombie nodes

