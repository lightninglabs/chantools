## chantools sweepremoteclosed

Go through all the addresses that could have funds of channels that were force-closed by the remote party. A public block explorer is queried for each address and if any balance is found, all funds are swept to a given address

### Synopsis

This command helps users sweep funds that are in 
outputs of channels that were force-closed by the remote party. This command
only needs to be used if no channel.backup file is available. By manually
contacting the remote peers and asking them to force-close the channels, the
funds can be swept after the force-close transaction was confirmed.

Supported remote force-closed channel types are:
 - STATIC_REMOTE_KEY (a.k.a. tweakless channels)
 - ANCHOR (a.k.a. anchor output channels)
 - SIMPLE_TAPROOT (a.k.a. simple taproot channels)


```
chantools sweepremoteclosed [flags]
```

### Examples

```
chantools sweepremoteclosed \
	--recoverywindow 300 \
	--feerate 20 \
	--sweepaddr bc1q..... \
  	--publish
```

### Options

```
      --apiurl string           API URL to use (must be esplora compatible) (default "https://api.node-recovery.com")
      --bip39                   read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
      --feerate uint32          fee rate to use for the sweep transaction in sat/vByte (default 30)
  -h, --help                    help for sweepremoteclosed
      --hsm_secret string       the hex encoded HSM secret to use for deriving the multisig keys for a CLN node; obtain by running 'xxd -p -c32 ~/.lightning/bitcoin/hsm_secret'
      --known_outputs string    a comma separated list of known output addresses to use for matching against, instead of querying the API; can also be a file name to a file that contains the known outputs, one per line
      --peers string            comma separated list of hex encoded public keys of the remote peers to recover funds from, only required when using --hsm_secret to derive the keys; can also be a file name to a file that contains the public keys, one per line
      --publish                 publish sweep TX to the chain API instead of just printing the TX
      --recoverywindow uint32   number of keys to scan per derivation path (default 200)
      --rootkey string          BIP32 HD root key of the wallet to use for sweeping the wallet; leave empty to prompt for lnd 24 word aezeed
      --sweepaddr string        address to recover the funds to; specify 'fromseed' to derive a new address from the seed automatically
      --walletdb string         read the seed/master root key to use for sweeping the wallet from an lnd wallet.db file instead of asking for a seed or providing the --rootkey flag
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

