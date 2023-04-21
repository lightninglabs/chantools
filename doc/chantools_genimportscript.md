## chantools genimportscript

Generate a script containing the on-chain keys of an lnd wallet that can be imported into other software like bitcoind

### Synopsis

Generates a script that contains all on-chain private (or
public) keys derived from an lnd 24 word aezeed wallet. That script can then be
imported into other software like bitcoind.

The following script formats are currently supported:
* bitcoin-cli: Creates a list of bitcoin-cli importprivkey commands that can
  be used in combination with a bitcoind full node to recover the funds locked
  in those private keys. NOTE: This will only work for legacy wallets and only
  for legacy, p2sh-segwit and bech32 (p2pkh, np2wkh and p2wkh) addresses. Use
  bitcoin-descriptors and a descriptor wallet for bech32m (p2tr).
* bitcoin-cli-watchonly: Does the same as bitcoin-cli but with the
  bitcoin-cli importpubkey command. That means, only the public keys are 
  imported into bitcoind to watch the UTXOs of those keys. The funds cannot be
  spent that way as they are watch-only.
* bitcoin-importwallet: Creates a text output that is compatible with
  bitcoind's importwallet command.
* electrum: Creates a text output that contains one private key per line with
  the address type as the prefix, the way Electrum expects them.
* bitcoin-descriptors: Create a list of bitcoin-cli importdescriptors commands
  that can be used in combination with a bitcoind full node that has a
  descriptor wallet to recover the funds locked in those private keys.
  NOTE: This will only work for descriptor wallets and only for
  p2sh-segwit, bech32 and bech32m (np2wkh, p2wkh and p2tr) addresses.

```
chantools genimportscript [flags]
```

### Examples

```
chantools genimportscript --format bitcoin-cli \
	--recoverywindow 5000
```

### Options

```
      --bip39                   read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
      --derivationpath string   use one specific derivation path; specify the first levels of the derivation path before any internal/external branch; Cannot be used in conjunction with --lndpaths
      --format string           format of the generated import script; currently supported are: bitcoin-importwallet, bitcoin-cli, bitcoin-cli-watchonly, bitcoin-descriptors and electrum (default "bitcoin-importwallet")
  -h, --help                    help for genimportscript
      --lndpaths                use all derivation paths that lnd used; results in a large number of results; cannot be used in conjunction with --derivationpath
      --recoverywindow uint32   number of keys to scan per internal/external branch; output will consist of double this amount of keys (default 2500)
      --rescanfrom uint32       block number to rescan from; will be set automatically from the wallet birthday if the lnd 24 word aezeed is entered (default 500000)
      --rootkey string          BIP32 HD root key of the wallet to use for decrypting the backup; leave empty to prompt for lnd 24 word aezeed
      --stdout                  write generated import script to standard out instead of writing it to a file
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

