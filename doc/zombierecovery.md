# Zombie Channel Recovery

## Read me first

It's useful to understand Zombie Channel recovery is one of the latest
"desparate" steps in recovering balances. Basically the channel states are not
known anymore: no one knows what the balances were (there is no channel DB,
static channel backup did not work, force closing was not possible).

This means:
1. Hou have to find the peer (which, if you want to do a zombie recovery, was
   probably offline if you tried to recover)
2. You have to find a way to contact this peer (twitter, email, ...)
3. You and your peer will have to negotiate a closing state: you only know the
   total channel size, and neither of you knows with good confidence who owes
   what. _(Because if you did, you would be able to restore using the static
   channel backup or channel DB.)_ There might be some guessing involved here,
   or maybe you want to do a 50:50 split.

## Recovery steps

Preparation:
* Make sure you have your seed words ready, you will need these to generate
  addresses and prepare keys
* *Got a request from a counterparty* to prepare keys? Skip to step 4 below.
* If you already have a way to contact your peer you can create your own match
  file (see ["File format" section](#file-format)) and skip to step 3 below.

Steps:

Below image is a simplified version of the steps described below the image.
![Zombierecovery Flow](zombierecovery-flow.svg)

1. _(Optional -- only needed if you have no contact with the remote party)_
   Register at [node-recovery.com](https://node-recovery.com). This website
   helps you connect to peers, for recovery. You enter your public key and some
   way to reach you. Hopefully, a peer with whom you have a "crashed" channel,
   will also register. The site will inform you of the peer's contact details,
   and will send channel information.
2. Got a message about a match? Congrats! You might be able to fix something
   together. Have a look in the recovery file, and look up the mentioned
   channel(s). Node-recovery has looked back at _their_ channel database and
   guessed if the channel needs to be recovered. Please check this, because if
   the guess was wrong, and the channel is active, you do not need to do this
   recovery at all and exit this guide! If you do not see it on your (recovered)
   node, continue:
3. Send/upload the JSON file(s) to your node. If you open the JSON file(s), you
   will see your own node ID (and contact info) and the peers'. [Download or
   install chantools](https://github.com/lightninglabs/chantools#installation).
   Technically, you do not _need_ to install `chantools` on the same machine as
   your node. Maybe you do not feel confident entering your seed words on your
   node and want to do this someplace else.
4. Prepare the keys. Both parties will need to do this. The payout address is a
   bitcoin address your sats will be sent to if you both agree on the offer
   (step 6). The final argument must be the match file, which you got in email.  
```
chantools zombierecovery preparekeys --payout_addr bc1xxx --match_file /tmp/match.json
```
5. The command will output a file in the `results/` folder (relative from where
   you ran the command, so after the command finished you can probably do
   `cd results/` to go to the results folder. Type `ls` to view the files. You
   can view the result in a text editor (`nano preparedkeys*`), it's regular
   readable text. It has the contact info of the counterparty. Now is a good
   time to contact the peer. You can attach your preparedkeys file and propose a
   fee rate (sat/vB). Ask them to prepare keys too.
6. Either your counterparty or you will make an offer. As explained in the
   ["Read me first" section](#read-me-first), there might be some guessing
   needed here. Basically the offer proposes a way to split the channel balance
   among you and your peer. If you sent your preparedkeysfile, your counterparty
   has enough information to create an offer. If they send their
   preparedkeysfile to you, you can create the offer. This has a split, which
   you might need to discuss with your peer. Either way, the command to advance
   to the next step is:
```
chantools zombierecovery makeoffer \
	--node1_keys preparedkeys-xxxx-xx-xx-<pubkey1>.json \
	--node2_keys preparedkeys-xxxx-xx-xx-<pubkey2>.json \
	--feerate 15
```
7. The output is a PSBT. It's signed by the party which created the offer. This
   must now be signed by the other party (thereby accepting the offer):
```
chantools zombierecovery signoffer \
	--psbt <offered_psbt_base64>
```
8. After signing, the transaction can be broadcast. From the PSBT (_partially
   signed_ bitcoin transaction), by signing, you have now (together) created a
   proper bitcoin transaction. An offer has been made, you have agreed on what
   split ("piece of the pie") goes to whom, created a transaction, signed it.
   This completed transaction can now be sent, for example using
   `bitcoin-cli sendrawtransaction`.

## File format

For reference, the file format of the "match" file is below. If you get a match
from node-recovery.com, the file looks like below example. It's human-readable
JSON, and you need to open the file to see the contact details. Use any plain
text editor to open the file.

Components:
* `node1` and `node2` are the peers of the channel, with contact details. This
  is not needed by `chantools`, you will use it to contact your peer.
* Then there is a list of channels, often just one though. It has metadata as
  seen by the network.
* As you are taking the steps above, the file format will be appended. For
  example, after step 4 (preparing keys), the file will have a list of
  `multisig_keys` for the node who prepared the keys.

```json
{
    "node1": {
        "identity_pubkey": "03xxxxxx",
        "contact": "contact information for node 1, not needed by chantools itself"
    },
    "node2": {
        "identity_pubkey": "03yyyyyy",
        "contact": "contact information for node 2, not needed by chantools itself"
    },
    "channels": [
        {
            "short_channel_id": "61xxxxxxxxxxxxx (optional, numerical channel ID, can be found on 1ml.com)",
            "chan_point": "<txid>:<output_index> (also called channel point on 1ml.com)",
            "address": "bc1q...... (the channel's output address on chain, find out by looking up the channel point on a block explorer)",
            "capacity": 123456
        }
    ]
}
```

It's encouraged to look at the file, and what the files look like after doing
each of the steps. 

## More info
_More info at the help output of `chantools zombierecovery --help` or the
generated [documentation for the zombierecovery
command](chantools_zombierecovery.md)._
