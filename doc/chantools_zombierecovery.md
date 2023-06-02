## chantools zombierecovery

Try rescuing funds stuck in channels with zombie nodes

### Synopsis

A sub command that hosts a set of further sub commands
to help with recovering funds tuck in zombie channels.

Please visit https://github.com/lightninglabs/chantools/blob/master/doc/zombierecovery.md
for more information on how to use these commands.

```
chantools zombierecovery [flags]
```

### Options

```
  -h, --help   help for zombierecovery
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels
* [chantools zombierecovery findmatches](chantools_zombierecovery_findmatches.md)	 - [0/3] Match maker only: Find matches between registered nodes
* [chantools zombierecovery makeoffer](chantools_zombierecovery_makeoffer.md)	 - [2/3] Make an offer on how to split the funds to recover
* [chantools zombierecovery preparekeys](chantools_zombierecovery_preparekeys.md)	 - [1/3] Prepare all public keys for a recovery attempt
* [chantools zombierecovery signoffer](chantools_zombierecovery_signoffer.md)	 - [3/3] Sign an offer sent by the remote peer to recover funds

