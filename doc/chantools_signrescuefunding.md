## chantools signrescuefunding

Rescue funds locked in a funding multisig output that never resulted in a proper channel; this is the command the remote node (the non-initiator) of the channel needs to run

```
chantools signrescuefunding [flags]
```

### Options

```
  -h, --help          help for signrescuefunding
      --psbt string   Partially Signed Bitcoin Transaction that was provided by the initiator of the channel to rescue
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

