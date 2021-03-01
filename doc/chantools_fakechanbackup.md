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

```
chantools fakechanbackup [flags]
```

### Examples

```
chantools fakechanbackup --rootkey xprvxxxxxxxxxx \
	--capacity 123456 \
	--channelpoint f39310xxxxxxxxxx:1 \
	--initiator \
	--remote_node_addr 022c260xxxxxxxx@213.174.150.1:9735 \
	--short_channel_id 566222x300x1 \
	--multi_file fake.backup
```

### Options

```
      --bip39                     read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
      --capacity uint             the channel's capacity in satoshis
      --channelpoint string       funding transaction outpoint of the channel to rescue (<txid>:<txindex>) as it is displayed on 1ml.com
  -h, --help                      help for fakechanbackup
      --initiator                 whether our node was the initiator (funder) of the channel
      --multi_file string         the fake channel backup file to create (default "results/fake-2021-03-01-10-12-23.backup")
      --remote_node_addr string   the remote node connection information in the format pubkey@host:port
      --rootkey string            BIP32 HD root key of the wallet to use for encrypting the backup; leave empty to prompt for lnd 24 word aezeed
      --short_channel_id string   the short channel ID in the format <blockheight>x<transactionindex>x<outputindex>
```

### Options inherited from parent commands

```
  -r, --regtest   Indicates if regtest parameters should be used
  -t, --testnet   Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

