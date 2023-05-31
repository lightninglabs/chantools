## chantools

Chantools helps recover funds from lightning channels

### Synopsis

This tool provides helper functions that can be used rescue
funds locked in lnd channels in case lnd itself cannot run properly anymore.
Complete documentation is available at https://github.com/lightninglabs/chantools/.

### Options

```
  -h, --help      help for chantools
  -r, --regtest   Indicates if regtest parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools chanbackup](chantools_chanbackup.md)	 - Create a channel.backup file from a channel database
* [chantools closepoolaccount](chantools_closepoolaccount.md)	 - Tries to close a Pool account that has expired
* [chantools compactdb](chantools_compactdb.md)	 - Create a copy of a channel.db file in safe/read-only mode
* [chantools deletepayments](chantools_deletepayments.md)	 - Remove all (failed) payments from a channel DB
* [chantools derivekey](chantools_derivekey.md)	 - Derive a key with a specific derivation path
* [chantools doublespendinputs](chantools_doublespendinputs.md)	 - Tries to double spend the given inputs by deriving the private for the address and sweeping the funds to the given address. This can only be used with inputs that belong to an lnd wallet.
* [chantools dropchannelgraph](chantools_dropchannelgraph.md)	 - Remove all graph related data from a channel DB
* [chantools dumpbackup](chantools_dumpbackup.md)	 - Dump the content of a channel.backup file
* [chantools dumpchannels](chantools_dumpchannels.md)	 - Dump all channel information from an lnd channel database
* [chantools fakechanbackup](chantools_fakechanbackup.md)	 - Fake a channel backup file to attempt fund recovery
* [chantools filterbackup](chantools_filterbackup.md)	 - Filter an lnd channel.backup file and remove certain channels
* [chantools fixoldbackup](chantools_fixoldbackup.md)	 - Fixes an old channel.backup file that is affected by the lnd issue #3881 (unable to derive shachain root key)
* [chantools forceclose](chantools_forceclose.md)	 - Force-close the last state that is in the channel.db provided
* [chantools genimportscript](chantools_genimportscript.md)	 - Generate a script containing the on-chain keys of an lnd wallet that can be imported into other software like bitcoind
* [chantools migratedb](chantools_migratedb.md)	 - Apply all recent lnd channel database migrations
* [chantools recoverloopin](chantools_recoverloopin.md)	 - Recover a loop in swap that the loop daemon is not able to sweep
* [chantools removechannel](chantools_removechannel.md)	 - Remove a single channel from the given channel DB
* [chantools rescueclosed](chantools_rescueclosed.md)	 - Try finding the private keys for funds that are in outputs of remotely force-closed channels
* [chantools rescuefunding](chantools_rescuefunding.md)	 - Rescue funds locked in a funding multisig output that never resulted in a proper channel; this is the command the initiator of the channel needs to run
* [chantools rescuetweakedkey](chantools_rescuetweakedkey.md)	 - Attempt to rescue funds locked in an address with a key that was affected by a specific bug in lnd
* [chantools showrootkey](chantools_showrootkey.md)	 - Extract and show the BIP32 HD root key from the 24 word lnd aezeed
* [chantools signrescuefunding](chantools_signrescuefunding.md)	 - Rescue funds locked in a funding multisig output that never resulted in a proper channel; this is the command the remote node (the non-initiator) of the channel needs to run
* [chantools summary](chantools_summary.md)	 - Compile a summary about the current state of channels
* [chantools sweepremoteclosed](chantools_sweepremoteclosed.md)	 - Go through all the addresses that could have funds of channels that were force-closed by the remote party. A public block explorer is queried for each address and if any balance is found, all funds are swept to a given address
* [chantools sweeptimelock](chantools_sweeptimelock.md)	 - Sweep the force-closed state after the time lock has expired
* [chantools sweeptimelockmanual](chantools_sweeptimelockmanual.md)	 - Sweep the force-closed state of a single channel manually if only a channel backup file is available
* [chantools triggerforceclose](chantools_triggerforceclose.md)	 - Connect to a peer and send a custom message to trigger a force close of the specified channel
* [chantools vanitygen](chantools_vanitygen.md)	 - Generate a seed with a custom lnd node identity public key that starts with the given prefix
* [chantools walletinfo](chantools_walletinfo.md)	 - Shows info about an lnd wallet.db file and optionally extracts the BIP32 HD root key
* [chantools zombierecovery](chantools_zombierecovery.md)	 - Try rescuing funds stuck in channels with zombie nodes

