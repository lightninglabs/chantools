# Channel tools

## Index

* [Installation](#installation)
* [Overview](#overview)
* [Commands](#commands)
  + [chanbackup](#chanbackup)
  + [compactdb](#compactdb)
  + [derivekey](#derivekey)
  + [dumpbackup](#dumpbackup)
  + [dumpchannels](#dumpchannels)
  + [filterbackup](#filterbackup)
  + [fixoldbackup](#fixoldbackup)
  + [genimportscript](#genimportscript)
  + [forceclose](#forceclose)
  + [rescueclosed](#rescueclosed)
  + [showrootkey](#showrootkey)
  + [summary](#summary)
  + [sweeptimelock](#sweeptimelock)
  + [walletinfo](#walletinfo)

This tool provides helper functions that can be used to rescue funds locked in
`lnd` channels in case `lnd` itself cannot run properly any more.

**WARNING**: This tool was specifically built for a certain rescue operation and
might not be well-suited for your use case. Or not all edge cases for your needs
are coded properly. Please look at the code to understand what it does before
you use it for anything serious.

**WARNING 2**: This tool will query public block explorer APIs for some of the
commands, your privacy might not be preserved. Use at your own risk or supply
a private API URL with `--apiurl`.

## Installation

To install this tool, make sure you have `go 1.13.x` (or later) and `make`
installed and run the following command:

```bash
make install
```

## Overview

```text
Usage:
  chantools [OPTIONS] <command>

Application Options:
      --testnet          Set to true if testnet parameters should be used.
      --apiurl=          API URL to use (must be esplora compatible). (default: https://blockstream.info/api)
      --listchannels=    The channel input is in the format of lncli's listchannels format. Specify '-' to read from stdin.
      --pendingchannels= The channel input is in the format of lncli's pendingchannels format. Specify '-' to read from stdin.
      --fromsummary=     The channel input is in the format of this tool's channel summary. Specify '-' to read from stdin.
      --fromchanneldb=   The channel input is in the format of an lnd channel.db file.

Help Options:
  -h, --help             Show this help message

Available commands:
  chanbackup       Create a channel.backup file from a channel database.
  compactdb        Open a source channel.db database file in safe/read-only mode and copy it to a fresh database, compacting it in the process.
  derivekey        Derive a key with a specific derivation path from the BIP32 HD root key.
  dumpbackup       Dump the content of a channel.backup file.
  dumpchannels     Dump all channel information from lnd's channel database.
  filterbackup     Filter an lnd channel.backup file and remove certain channels.
  fixoldbackup     Fixes an old channel.backup file that is affected by the lnd issue #3881 (unable to derive shachain root key).
  forceclose       Force-close the last state that is in the channel.db provided.
  genimportscript  Generate a script containing the on-chain keys of an lnd wallet that can be imported into other software like bitcoind.
  rescueclosed     Try finding the private keys for funds that are in outputs of remotely force-closed channels.
  showrootkey      Extract and show the BIP32 HD root key from the 24 word lnd aezeed.
  summary          Compile a summary about the current state of channels.
  sweeptimelock    Sweep the force-closed state after the time lock has expired.
  walletinfo       Shows relevant information about an lnd wallet.db file and optionally extracts the BIP32 HD root key.
```

## Commands

### chanbackup

```text
Usage:
  chantools [OPTIONS] chanbackup [chanbackup-OPTIONS]

[chanbackup command options]
          --rootkey=     BIP32 HD root key of the wallet that should be used to create the backup. Leave empty to prompt for lnd 24 word aezeed.
          --channeldb=   The lnd channel.db file to create the backup from.
          --multi_file=  The lnd channel.backup file to create.
```

This command creates a new channel.backup from a channel.db file.

Example command:

```bash
chantools chanbackup --rootkey xprvxxxxxxxxxx \
  --channeldb ~/.lnd/data/graph/mainnet/channel.db \
  --multi_file new_channel_backup.backup 
```

### compactdb

```text
Usage:
  chantools [OPTIONS] compactdb [compactdb-OPTIONS]

[compactdb command options]
          --txmaxsize=   Maximum transaction size. (default 65536)
          --sourcedb=    The lnd channel.db file to create the database backup from.
          --destdb=      The lnd new channel.db file to copy the compacted database to.
```

This command opens a database in read-only mode and tries to create a copy of it
to a destination file, compacting it in the process.

Example command:

```bash
chantools compactdb --sourcedb ~/.lnd/data/graph/mainnet/channel.db \
  --destdb ./results/compacted.db
```

### derivekey

```text
Usage:
  chantools [OPTIONS] derivekey [derivekey-OPTIONS]

[derivekey command options]
          --rootkey=     BIP32 HD root key to derive the key from. Leave empty to prompt for lnd 24 word aezeed.
          --path=        The BIP32 derivation path to derive. Must start with "m/".
          --neuter       Do not output the private key, just the public key.
```

This command derives a single key with the given BIP32 derivation path from the
root key and prints it to the console. Make sure to escape apostrophes in the
derivation path.

Example command:

```bash
chantools derivekey --rootkey xprvxxxxxxxxxx --path m/1017\'/0\'/5\'/0/0 \
  --neuter
```

### dumpbackup

```text
Usage:
  chantools [OPTIONS] dumpbackup [dumpbackup-OPTIONS]

[dumpbackup command options]
          --rootkey=     BIP32 HD root key of the wallet that was used to create the backup. Leave empty to prompt for lnd 24 word aezeed.
          --multi_file=  The lnd channel.backup file to dump.
```

This command dumps all information that is inside a `channel.backup` file in a
human readable format.

Example command:

```bash
chantools dumpbackup --rootkey xprvxxxxxxxxxx \
  --multi_file ~/.lnd/data/chain/bitcoin/mainnet/channel.backup
```

### dumpchannels

```text
Usage:
  chantools [OPTIONS] dumpchannels [dumpchannels-OPTIONS]

[dumpchannels command options]
          --channeldb=   The lnd channel.db file to dump the channels from.
```

This command dumps all open and pending channels from the given lnd `channel.db`
file in a human readable format.

Example command:

```bash
chantools dumpchannels --channeldb ~/.lnd/data/graph/mainnet/channel.db
```

### filterbackup

```text
Usage:
  chantools [OPTIONS] filterbackup [filterbackup-OPTIONS]

[filterbackup command options]
          --rootkey=     BIP32 HD root key of the wallet that was used to create the backup. Leave empty to prompt for lnd 24 word aezeed.
          --multi_file=  The lnd channel.backup file to filter.
          --discard=     A comma separated list of channel funding outpoints (format <fundingTXID>:<index>) to remove from the backup file.
```

Filter an `lnd` `channel.backup` file by removing certain channels (identified by
their funding transaction outpoints). 

Example command:

```bash
chantools filterbackup --rootkey xprvxxxxxxxxxx \
  --multi_file ~/.lnd/data/chain/bitcoin/mainnet/channel.backup \
  --discard 2abcdef2b2bffaaa...db0abadd:1,4abcdef2b2bffaaa...db8abadd:0
```

### fixoldbackup

```text
Usage:
  chantools [OPTIONS] fixoldbackup [fixoldbackup-OPTIONS]

[fixoldbackup command options]
          --rootkey=     BIP32 HD root key of the wallet that was used to create the backup. Leave empty to prompt for lnd 24 word aezeed.
          --multi_file=  The lnd channel.backup file to fix.
```

Fixes an old channel.backup file that is affected by the `lnd` issue
[#3881](https://github.com/lightningnetwork/lnd/issues/3881) (<code>[lncli]
unable to restore chan backups: rpc error: code = Unknown desc = unable
to unpack chan backup: unable to derive shachain root key: unable to derive
private key</code>).

Example command:

```bash
chantools fixoldbackup --rootkey xprvxxxxxxxxxx \
  --multi_file ~/.lnd/data/chain/bitcoin/mainnet/channel.backup
```

### forceclose

```text
Usage:
  chantools [OPTIONS] forceclose [forceclose-OPTIONS]

[forceclose command options]
          --rootkey=     BIP32 HD root key to use. Leave empty to prompt for lnd 24 word aezeed.
          --channeldb=   The lnd channel.db file to use for force-closing channels.
          --publish      Should the force-closing TX be published to the chain API?
```

If you are certain that a node is offline for good (AFTER you've tried SCB!) and
a channel is still open, you can use this method to force-close your latest
state that you have in your channel.db.

**!!! WARNING !!! DANGER !!! WARNING !!!**

If you do this and the state that you publish is *not* the latest state, then
the remote node *could* punish you by taking the whole channel amount *if* they
come online before you can sweep the funds from the time locked (144 - 2000
blocks) transaction *or* they have a watch tower looking out for them.

**This should absolutely be the last resort and you have been warned!**

Example command:

```bash
chantools --fromsummary results/summary-xxxx-yyyy.json \
  forceclose \
  --channeldb ~/.lnd/data/graph/mainnet/channel.db \
  --rootkey xprvxxxxxxxxxx \
  --publish
```

### genimportscript

```text
Usage:
  chantools [OPTIONS] genimportscript [genimportscript-OPTIONS]

[genimportscript command options]
          --rootkey=        BIP32 HD root key to use. Leave empty to prompt for lnd 24 word aezeed.
          --format=         The format of the generated import script. Currently supported are: bitcoin-cli, bitcoin-cli-watchonly.
          --recoverywindow= The number of keys to scan per internal/external branch. The output will consist of double this amount of keys. (default 2500)
          --rescanfrom=     The block number to rescan from. Will be set automatically from the wallet birthday if the lnd 24 word aezeed is entered. (default 500000)
```

Generates a script that contains all on-chain private (or public) keys derived
from an `lnd` 24 word aezeed wallet. That script can then be imported into other
software like bitcoind.

The following script formats are currently supported:
* `bitcoin-cli`: Creates a list of `bitcoin-cli importprivkey` commands that can
  be used in combination with a `bitcoind` full node to recover the funds locked
  in those private keys.
* `bitcoin-cli-watchonly`: Does the same as `bitcoin-cli` but with the
  `bitcoin-cli importpubkey` command. That means, only the public keys are 
  imported into `bitcoind` to watch the UTXOs of those keys. The funds cannot be
  spent that way as they are watch-only.
* `bitcoin-importwallet`: Creates a text output that is compatible with
  `bitcoind`'s `importwallet command.

Example command:

```bash
chantools genimportscript --format bitcoin-cli --recoverywindow 5000
```

### rescueclosed

```text
Usage:
  chantools [OPTIONS] rescueclosed [rescueclosed-OPTIONS]

[rescueclosed command options]
          --rootkey=     BIP32 HD root key to use. Leave empty to prompt for lnd 24 word aezeed.
          --channeldb=   The lnd channel.db file to use for rescuing force-closed channels.
```

If channels have already been force-closed by the remote peer, this command
tries to find the private keys to sweep the funds from the output that belongs
to our side. This can only be used if we have a channel DB that contains the
latest commit point. Normally you would use SCB to get the funds from those
channels. But this method can help if the other node doesn't know about the
channels any more but we still have the channel.db from the moment they
force-closed.

Example command:

```bash
chantools --fromsummary results/summary-xxxx-yyyy.json \
  rescueclosed \
  --channeldb ~/.lnd/data/graph/mainnet/channel.db \
  --rootkey xprvxxxxxxxxxx
```

### showrootkey

This command converts the 24 word `lnd` aezeed phrase and password to the BIP32
HD root key that is used as the `rootkey` parameter in other commands of this
tool.

Example command:

```bash
chantools showrootkey
```

### summary

```text
Usage:
  chantools [OPTIONS] summary
```

From a list of channels, find out what their state is by querying the funding
transaction on a block explorer API.

Example command 1:

```bash
lncli listchannels | chantools --listchannels - summary
```

Example command 2:

```bash
chantools --fromchanneldb ~/.lnd/data/graph/mainnet/channel.db
```

### sweeptimelock

```text
Usage:
  chantools [OPTIONS] sweeptimelock [sweeptimelock-OPTIONS]

[sweeptimelock command options]
          --rootkey=     BIP32 HD root key to use. Leave empty to prompt for lnd 24 word aezeed.
          --publish      Should the sweep TX be published to the chain API?
          --sweepaddr=   The address the funds should be sweeped to
          --maxcsvlimit= Maximum CSV limit to use. (default 2000)
```

Use this command to sweep the funds from channels that you force-closed with the
`forceclose` command. You **MUST** use the result file that was created with the
`forceclose` command, otherwise it won't work. You also have to wait until the
highest time lock (can be up to 2000 blocks which is more than two weeks) of all
the channels has passed. If you only want to sweep channels that have the
default CSV limit of 1 day, you can set the `--maxcsvlimit` parameter to 144.

Example command:

```bash
chantools --fromsummary results/forceclose-xxxx-yyyy.json \
  sweeptimelock
  --rootkey xprvxxxxxxxxxx \
  --publish \
  --sweepaddr bc1q.....
```

### walletinfo

```text
Usage:
  chantools [OPTIONS] walletinfo [walletinfo-OPTIONS]

[walletinfo command options]
          --walletdb=    The lnd wallet.db file to dump the contents from.
          --withrootkey  Should the BIP32 HD root key of the wallet be printed to standard out?
```

Shows some basic information about an `lnd` `wallet.db` file, like the node
identity the wallet belongs to, how many on-chain addresses are used and, if
enabled with `--withrootkey` the BIP32 HD root key of the wallet. The latter can
be useful to recover funds from a wallet if the wallet password is still known
but the seed was lost. **The 24 word seed phrase itself cannot be extracted** 
because it is hashed into the extended HD root key before storing it in the
`wallet.db`.

Example command:

```bash
chantools walletinfo \
  --walletdb ~/.lnd/data/chain/bitcoin/mainnet/wallet.db \
  --withrootkey
```
