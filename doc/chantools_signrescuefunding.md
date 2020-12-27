## chantools signrescuefunding

Rescue funds locked in a funding multisig output that never resulted in a proper channel; this is the command the remote node (the non-initiator) of the channel needs to run

### Synopsis

This is part 2 of a two phase process to rescue a channel
funding output that was created on chain by accident but never resulted in a
proper channel and no commitment transactions exist to spend the funds locked in
the 2-of-2 multisig.

If successful, this will create a final on-chain transaction that can be
broadcast by any Bitcoin node.

```
chantools signrescuefunding [flags]
```

### Examples

```
chantools signrescuefunding --rootkey xprvxxxxxxxxxx \
	--psbt <the_base64_encoded_psbt_from_step_1>
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

