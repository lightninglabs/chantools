## chantools walletinfo

Shows info about an lnd wallet.db file and optionally extracts the BIP32 HD root key

### Synopsis

Shows some basic information about an lnd wallet.db file,
like the node identity the wallet belongs to, how many on-chain addresses are
used and, if enabled with --withrootkey the BIP32 HD root key of the wallet. The
latter can be useful to recover funds from a wallet if the wallet password is
still known but the seed was lost. **The 24 word seed phrase itself cannot be
extracted** because it is hashed into the extended HD root key before storing it
in the wallet.db.
In case lnd was started with "--noseedbackup=true" your wallet has the default
password. To unlock the wallet set the environment variable WALLET_PASSWORD="-"
or simply press <enter> without entering a password when being prompted.

```
chantools walletinfo [flags]
```

### Examples

```
chantools walletinfo --withrootkey \
	--walletdb ~/.lnd/data/chain/bitcoin/mainnet/wallet.db
```

### Options

```
      --dumpaddrs         print all addresses, including private keys
  -h, --help              help for walletinfo
      --walletdb string   lnd wallet.db file to dump the contents from
      --withrootkey       print BIP32 HD root key of wallet to standard out
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -s, --signet    Indicates if the public signet parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

