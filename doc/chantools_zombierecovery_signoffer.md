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
      --apiurl string        API URL to use for publishing the final transaction (must be esplora compatible) (default "https://api.node-recovery.com")
      --bip39                read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
  -h, --help                 help for signoffer
      --hsm_secret string    the hex encoded HSM secret to use for deriving the multisig keys for a CLN node; obtain by running 'xxd -p -c32 ~/.lightning/bitcoin/hsm_secret'
      --psbt string          the base64 encoded PSBT that the other party sent as an offer to rescue funds
      --publish              if set, the final PSBT will be published to the network after signing, otherwise it will just be printed to stdout
      --remote_peer string   the hex encoded remote peer node identity key, only required when running 'signoffer' on the CLN side
      --rootkey string       BIP32 HD root key of the wallet to use for signing the offer; leave empty to prompt for lnd 24 word aezeed
      --walletdb string      read the seed/master root key to use for signing the offer from an lnd wallet.db file instead of asking for a seed or providing the --rootkey flag
```

### Options inherited from parent commands

```
      --nologfile   If set, no log file will be created. This is useful for testing purposes where we don't want to create a log file.
  -r, --regtest     Indicates if regtest parameters should be used
  -s, --signet      Indicates if the public signet parameters should be used
  -t, --testnet     Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools zombierecovery](chantools_zombierecovery.md)	 - Try rescuing funds stuck in channels with zombie nodes

