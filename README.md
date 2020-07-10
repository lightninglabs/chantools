# Channel tools

## Index

* [Installation](#installation)
* [Command overview](#command-overview)
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
`lnd` channels in case `lnd` itself cannot run properly anymore.

**WARNING**: This tool was specifically built for a certain rescue operation and
might not be well-suited for your use case. Or not all edge cases for your needs
are coded properly. Please look at the code to understand what it does before
you use it for anything serious.

**WARNING 2**: This tool will query public block explorer APIs for some
commands, your privacy might not be preserved. Use at your own risk or supply
a private API URL with `--apiurl`.

## Installation

To install this tool, make sure you have `go 1.13.x` (or later) and `make`
installed and run the following commands:

```bash
git clone https://github.com/guggero/chantools.git
cd chantools
make install
```

## Channel recovery scenario

The following flow chart shows the main recovery scenario this tool was built
for. This scenario assumes that you do have access to the crashed node's seed,
`channel.backup` file and some state of a `channel.db` file (perhaps from a
file based backup or the recovered file from the crashed node).

![rescue flow](doc/rescue-flow.png)

**Explanation:**

1. **Node crashed**: For some reason your `lnd` node crashed and isn't starting
  anymore. If you get errors similar to
  [this](https://github.com/lightningnetwork/lnd/issues/4449),
  [this](https://github.com/lightningnetwork/lnd/issues/3473) or
  [this](https://github.com/lightningnetwork/lnd/issues/4102), it is possible
  that a simple compaction (a full copy in safe mode) can solve your problem.
  See [`chantools compactdb`](#compactdb).
  <br/><br/>
  If that doesn't work and you need to continue the recovery, make sure you can
  at least extract the `channel.backup` file and if somehow possible any version
  of the `channel.db` from the node.
  <br/><br/>
  Whatever you do, do **never, ever** replace your `channel.db` file with an old
  version (from a file based backup) and start your node that way. [Read this
  explanation why that can lead to loss of funds.](https://github.com/lightningnetwork/lnd/blob/master/docs/safety.md#file-based-backups)

2. **Rescue on-chain balance**: To start the recovery process, we are going to
  re-create the node from scratch. To make sure we don't overwrite any old data
  in the process, make sure the old data directory of your node (usually `.lnd`
  in the user's home directory) is safely moved away (or the whole folder
  renamed) before continuing.
  <br/>
  To start the on-chain recovery, [follow the sub step "Starting On-Chain
  Recovery" of this guide](https://github.com/lightningnetwork/lnd/blob/master/docs/recovery.md#starting-on-chain-recovery).
  Don't follow the whole guide, only this single chapter!
  <br/><br/>
  This step is completed once the `lncli getinfo` command shows both
  `"synced_to_chain": true` and `"synced_to_graph": true` which can take several
  hours depending on the speed of your hardware. **Do not be alarmed** that the
  `lncli getinfo` command shows 0 channels. This is normal as we haven't started
  the off-chain recovery yet.

3. **Recover channels using SCB**: Now that the node is fully synced, we can try
  to recover the channels using the [Static Channel Backups (SCB)](https://github.com/lightningnetwork/lnd/blob/master/docs/safety.md#static-channel-backups-scbs).
  For this, you need a file called `channel.backup`. Simply run the command
  `lncli restorechanbackup --multi_file <path-to-your-channel.backup>`. **This
  will take a while!**. The command itself can take several minutes to complete,
  depending on the number of channels. The recovery can easily take a day or
  two as a lot of chain rescanning needs to happen. It is recommended to wait at
  least one full day. You can watch the progress with the `lncli pendingchannels`
  command. If the list is empty, congratulations, you've recovered all channels!
  If the list stays un-changed for several hours, it means not all channels
  could be restored using this method.
  [One explanation can be found here.](https://github.com/lightningnetwork/lnd/blob/master/docs/safety.md#zombie-channels)

4. **Install chantools**: To try to recover the remaining channels, we are going
  to use `chantools`. Simply [follow the installation instructions.](#installation)
  The recovery can only be continued if you have access to some version of the
  crashed node's `channel.db`. This could be the latest state as recovered from
  the crashed file system, or a version from a regular file based backup. If you
  do not have any version of a channel DB, `chantools` won't be able to help
  with the recovery. See step 11 for some possible manual steps.

5. **Create copy of channel DB**: To make sure we can read the channel DB, we
  are going to create a copy in safe mode (called compaction). Simply run
  <br/><br/>
  `chantools compactdb --sourcedb <recovered-channel.db> --destdb ./results/compacted.db`
  <br/><br/>
  We are going to assume that the compacted copy of the channel DB is located in
  `./results/compacted.db` in the following commands.

6. **chantools summary**: First, `chantools` needs to find out the state of each
  channel on chain. For this, a blockchain API (by default [blockstream.info](https://blockstream.info))
  is queried. The result will be written to a file called
  `./results/summary-yyyy-mm-dd.json`. This result file will be needed for the
  next command.
  <br/><br/>
  `chantools --fromchanneldb ./results/compacted.db summary`

7. **chantools rescueclosed**: It is possible that by now the remote peers have
  force-closed some of the remaining channels. What we now do is try to find the
  private keys to sweep our balance of those channels. For this we need a shared
  secret which is called the `commit_point` and is changed whenever a channel is
  updated. We do have the latest known version of this point in the channel DB.
  The following command tries to find all private keys for channels that have
  been closed by the other party. The command needs to know what channels it is
  operating on, so we have to supply the `summary-yyy-mm-dd.json` created by the
  previous command:
  <br/><br/>
  `chantools --fromsummary ./results/<summary-file-created-in-last-step>.json rescueclosed --channeldb ./results/compacted.db`
  <br/><br/>
  This will create a new file called `./results/rescueclosed-yyyy-mm-dd.json`
  which will contain any found private keys and will also be needed for the next
  command. Use `bitcoind` or Electrum Wallet to sweep all of the private keys.

8. **chantools forceclose**: This command will now close all channels that
  `chantools` thinks are still open. This is achieved by publishing the latest
  known channel state of the `channel.db` file.
  <br/>**Please read the full warning text of the
  [`forceclose` command below](#forceclose) as this command can put
  your funds at risk** if the state in the channel DB is not the most recent
  one. This command should only be executed for channels where the remote peer
  is not online anymore.
  <br/><br/>
  `chantools --fromsummary ./results/<rescueclosed-file-created-in-last-step>.json forceclose --channeldb ./results/compacted.db --publish`
  <br/><br/>
  This will create a new file called `./results/forceclose-yyyy-mm-dd.json`
  which will be needed for the next command.

9. **Wait for timelocks**: The previous command closed the remaining open
  channels by publishing your node's state of the channel. By design of the
  Lightning Network, you now have to wait until the channel funds belonging to
  you are not time locked any longer. Depending on the size of the channel, you
  have to wait for somewhere between 144 and 2000 confirmations of the
  force-close transactions. Only continue with the next step after the channel
  with the highest `csv_timeout` has reached that many confirmations of its
  closing transaction.

10. **chantools sweeptimelock**: Once all force-close transactions have reached
  the number of transactions as the `csv_timeout` in the JSON demands, these
  time locked funds can now be swept. Use the following command to sweep all the
  channel funds to an address of your wallet:
  <br/><br/>
  `chantools --fromsummary ./results/<forceclose-file-created-in-last-step>.json sweeptimelock --publish --sweepaddr <bech32-address-from-your-wallet>`

11. **Manual intervention necessary**: You got to this step because you either
  don't have a `channel.db` file or because `chantools` couldn't rescue all your
  node's channels. There are a few things you can try manually that have some
  chance of working:
  - Make sure you can connect to all nodes when restoring from SCB: It happens
    all the time that nodes change their IP addresses. When restoring from a
    static channel backup, your node tries to connect to the node using the IP
    address encoded in the backup file. If the address changed, the SCB restore
    process doesn't work. You can use block explorers like [1ml.com](https://1ml.com)
    to try to find an IP address that is up-to-date. Just run
    `lncli connect <node-pubkey>@<updated-ip-address>:<port>` in the recovered
    `lnd` node from step 3 and wait a few hours to see if the channel is now
    being force closed by the remote node.
  - Find out who the node belongs to: Maybe you opened the channel with someone
    you know. Or maybe their node alias contains some information about who the
    node belongs to. If you can find out who operates the remote node, you can
    ask them to force-close the channel from your end. If the channel was opened
    with the `option_static_remote_key`, (`lnd v0.8.0` and later), the funds can
    be swept by your node.

## Command overview

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
          --format=         The format of the generated import script. Currently supported are: bitcoin-cli, bitcoin-cli-watchonly, bitcoin-importwallet.
          --lndpaths        Use all derivation paths that lnd uses. Results in a large number of results. Cannot be used in conjunction with --derivationpath.
          --derivationpath= Use one specific derivation path. Specify the first levels of the derivation path before any internal/external branch. Cannot be used in conjunction with --lndpaths. (default m/84'/0'/0')
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
