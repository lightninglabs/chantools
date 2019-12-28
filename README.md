# Channel tools

This tool provides helper functions that can be used to rescue funds locked in
lnd channels in case lnd itself cannot run properly any more.

**WARNING**: This tool was specifically built for a certain rescue operation and
might not be well-suited for your use case. Or not all edge cases for your needs
are coded properly. Please look at the code to understand what it does before
you use it for anything serious.

**WARNING 2**: This tool will query public block explorer APIs, your privacy
might not be preserved. Use at your own risk.

## Overview

```text
Usage:
  chantools [OPTIONS] <command>

Application Options:
      --apiurl=          API URL to use (must be esplora compatible). (default: https://blockstream.info/api)
      --listchannels=    The channel input is in the format of lncli's listchannels format. Specify '-' to read from stdin.
      --pendingchannels= The channel input is in the format of lncli's pendingchannels format. Specify '-' to read from stdin.
      --fromsummary=     The channel input is in the format of this tool's channel summary. Specify '-' to read from stdin.
      --fromchanneldb=   The channel input is in the format of an lnd channel.db file.

Help Options:
  -h, --help             Show this help message

Available commands:
  dumpchannels   Dump all channel information from lnd's channel database
  forceclose     Force-close the last state that is in the channel.db provided
  rescueclosed   Try finding the private keys for funds that are in outputs of remotely force-closed channels
  summary        Compile a summary about the current state of channels
  sweeptimelock  Sweep the force-closed state after the time lock has expired
```

## summary command

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

## rescueclosed command

```text
Usage:
  chantools [OPTIONS] rescueclosed [rescueclosed-OPTIONS]

[rescueclosed command options]
          --rootkey=     BIP32 HD root key to use.
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

## forceclose command

```text
Usage:
  chantools [OPTIONS] forceclose [forceclose-OPTIONS]

[forceclose command options]
          --rootkey=     BIP32 HD root key to use.
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

## sweeptimelock command

```text
Usage:
  chantools [OPTIONS] sweeptimelock [sweeptimelock-OPTIONS]

[sweeptimelock command options]
          --rootkey=     BIP32 HD root key to use.
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

## dumpchannels command

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
