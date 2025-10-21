## chantools closepoolaccount

Tries to close a Pool account that has expired

### Synopsis

In case a Pool account cannot be closed normally with the
poold daemon it can be closed with this command. The account **MUST** have
expired already, otherwise this command doesn't work since a signature from the
auctioneer is necessary.

You need to know the account's last unspent outpoint. That can either be
obtained by running 'pool accounts list' 

```
chantools closepoolaccount [flags]
```

### Examples

```
chantools closepoolaccount \
	--outpoint xxxxxxxxx:y \
	--sweepaddr bc1q..... \
	--feerate 10 \
  	--publish
```

### Options

```
      --apiurl string            API URL to use (must be esplora compatible) (default "https://api.node-recovery.com")
      --auctioneerkey string     the auctioneer's static public key (default "028e87bdd134238f8347f845d9ecc827b843d0d1e27cdcb46da704d916613f4fce")
      --bip39                    read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
      --feerate uint32           fee rate to use for the sweep transaction in sat/vByte (default 30)
  -h, --help                     help for closepoolaccount
      --maxnumaccounts uint32    the number of account indices to try at most (default 20)
      --maxnumbatchkeys uint32   the number of batch keys to try at most (default 500)
      --maxnumblocks uint32      the maximum number of blocks to try when brute forcing the expiry (default 200000)
      --minexpiry uint32         the block to start brute forcing the expiry from (default 648168)
      --outpoint string          last account outpoint of the account to close (<txid>:<txindex>)
      --publish                  publish sweep TX to the chain API instead of just printing the TX
      --rootkey string           BIP32 HD root key of the wallet to use for deriving keys; leave empty to prompt for lnd 24 word aezeed
      --sweepaddr string         address to recover the funds to; specify 'fromseed' to derive a new address from the seed automatically
      --walletdb string          read the seed/master root key to use for deriving keys from an lnd wallet.db file instead of asking for a seed or providing the --rootkey flag
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

