## chantools fakechanbackup

Fake a channel backup file to attempt fund recovery

### Synopsis

If for any reason a node suffers from data loss and there is no
channel.backup for one or more channels, then the funds in the channel would
theoretically be lost forever.
If the remote node is still online and still knows about the channel, there is
hope. We can initiate DLP (Data Loss Protocol) and ask the remote node to
force-close the channel and to provide us with the per_commit_point that is
needed to derive the private key for our part of the force-close transaction
output. But to initiate DLP, we would need to have a channel.backup file.
Fortunately, if we have enough information about the channel, we can create a
faked/skeleton channel.backup file that at least lets us talk to the other node
and ask them to do their part. Then we can later brute-force the private key for
the transaction output of our part of the funds (see rescueclosed command).

There are two versions of this command: The first one is to create a fake
backup for a single channel where all flags (except --from_channel_graph) need
to be set. This is the easiest to use since it only relies on data that is
publicly available (for example on 1ml.com) but involves more manual work.
The second version of the command only takes the --from_channel_graph and
--multi_file flags and tries to assemble all channels found in the public
network graph (must be provided in the JSON format that the 
'lncli describegraph' command returns) into a fake backup file. This is the
most convenient way to use this command but requires one to have a fully synced
lnd node.

Any fake channel backup _needs_ to be used with the custom fork of lnd
specifically built for this purpose: https://github.com/guggero/lnd/releases
Also the debuglevel must be set to debug (lnd.conf, set 'debuglevel=debug') when
running the above lnd for it to produce the correct log file that will be needed
for the rescueclosed command.


```
chantools fakechanbackup [flags]
```

### Examples

```
chantools fakechanbackup \
	--capacity 123456 \
	--channelpoint f39310xxxxxxxxxx:1 \
	--remote_node_addr 022c260xxxxxxxx@213.174.150.1:9735 \
	--short_channel_id 566222x300x1 \
	--multi_file fake.backup

chantools fakechanbackup --from_channel_graph lncli_describegraph.json \
	--multi_file fake.backup
```

### Options

```
      --bip39                       read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
      --capacity uint               the channel's capacity in satoshis
      --channelpoint string         funding transaction outpoint of the channel to rescue (<txid>:<txindex>) as it is displayed on 1ml.com
      --from_channel_graph string   the full LN channel graph in the JSON format that the 'lncli describegraph' returns
  -h, --help                        help for fakechanbackup
      --multi_file string           the fake channel backup file to create (default "results/fake-2024-01-26-02-27-52.backup")
      --remote_node_addr string     the remote node connection information in the format pubkey@host:port
      --rootkey string              BIP32 HD root key of the wallet to use for encrypting the backup; leave empty to prompt for lnd 24 word aezeed
      --short_channel_id string     the short channel ID in the format <blockheight>x<transactionindex>x<outputindex>
      --walletdb string             read the seed/master root key to use fro encrypting the backup from an lnd wallet.db file instead of asking for a seed or providing the --rootkey flag
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -s, --signet    Indicates if the public signet parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

