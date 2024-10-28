## chantools recoverloopin

Recover a loop in swap that the loop daemon is not able to sweep

```
chantools recoverloopin [flags]
```

### Examples

```
chantools recoverloopin \
	--txid abcdef01234... \
	--vout 0 \
	--swap_hash abcdef01234... \
	--loop_db_dir /path/to/loop/db/dir \
	--sweep_addr bc1pxxxxxxx \
	--feerate 10
```

### Options

```
      --apiurl string         API URL to use (must be esplora compatible) (default "https://api.node-recovery.com")
      --bip39                 read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
      --feerate uint32        fee rate to use for the sweep transaction in sat/vByte
  -h, --help                  help for recoverloopin
      --loop_db_dir string    path to the loop database directory, where the loop.db file is located
      --num_tries int         number of tries to try to find the correct key index (default 1000)
      --output_amt uint       amount of the output to sweep
      --publish               publish sweep TX to the chain API instead of just printing the TX
      --rootkey string        BIP32 HD root key of the wallet to use for deriving starting key; leave empty to prompt for lnd 24 word aezeed
      --sqlite_file string    optional path to the loop sqlite database file, if not specified, the default location will be loaded from --loop_db_dir
      --start_key_index int   start key index to try to find the correct key index
      --swap_hash string      swap hash of the loop in swap
      --sweepaddr string      address to recover the funds to; specify 'fromseed' to derive a new address from the seed automatically
      --txid string           transaction id of the on-chain transaction that created the HTLC
      --vout uint32           output index of the on-chain transaction that created the HTLC
      --walletdb string       read the seed/master root key to use fro deriving starting key from an lnd wallet.db file instead of asking for a seed or providing the --rootkey flag
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -s, --signet    Indicates if the public signet parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

