## chantools zombierecovery signoffer

[3/3] Sign an offer sent by the remote peer to recover funds

### Synopsis

Inspect and sign an offer that was sent by the remote
peer to recover funds from one or more channels.

```
chantools zombierecovery signoffer [flags]
```

### Examples

```
chantools zombierecovery signoffer \
	--psbt <offered_psbt_base64>
```

### Options

```
      --bip39            read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
  -h, --help             help for signoffer
      --psbt string      the base64 encoded PSBT that the other party sent as an offer to rescue funds
      --rootkey string   BIP32 HD root key of the wallet to use for signing the offer; leave empty to prompt for lnd 24 word aezeed
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools zombierecovery](chantools_zombierecovery.md)	 - Try rescuing funds stuck in channels with zombie nodes

