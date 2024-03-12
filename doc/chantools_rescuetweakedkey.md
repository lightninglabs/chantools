## chantools rescuetweakedkey

Attempt to rescue funds locked in an address with a key that was affected by a specific bug in lnd

### Synopsis

There very likely is no reason to run this command 
unless you exactly know why or were told by the author of this tool to use it.


```
chantools rescuetweakedkey [flags]
```

### Examples

```
chantools rescuetweakedkey \
	--path "m/1017'/0'/5'/0/0'" \
	--targetaddr bc1pxxxxxxx
```

### Options

```
      --bip39               read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
  -h, --help                help for rescuetweakedkey
      --numtries uint       the number of mutations to try (default 10000000)
      --path string         BIP32 derivation path to derive the starting key from; must start with "m/"
      --rootkey string      BIP32 HD root key of the wallet to use for deriving starting key; leave empty to prompt for lnd 24 word aezeed
      --targetaddr string   address the funds are locked in
      --walletdb string     read the seed/master root key to use fro deriving starting key from an lnd wallet.db file instead of asking for a seed or providing the --rootkey flag
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -s, --signet    Indicates if the public signet parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

