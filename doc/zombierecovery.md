# Zombie Channel Recovery

* Got a request from a counterparty to prepare keys? Skip to step 4
* Do you have contact with a peer you can skip to step 3.

## Recovery steps
1. _(Optional -- only needed if you have no contact with the remote party)_ Register at [node-recovery.com](https://node-recovery.com).
2. Got a message about a match? Congrats! You might be able to fix something together.
3. Send the JSON file(s) to your node. If you open the JSON file(s), you will see your own node ID (and contact info) and the peers'. [Download or install chantools](https://github.com/guggero/chantools#installation).
4. Prepare the keys. Both parties will need to do this.
```
./chantools zombierecovery preparekeys --payout_addr bc1xxx --match_file /tmp/match.json
```
5. You can view the result. It has the contact info of the counterparty. Now is a good time to contact the peer. You can attach your preparedkeys file and propose a fee rate (sat/vB). Ask them to prepare keys too.
6. Either your counterparty or you will make an offer. If you sent your preparedkeysfile, your counterparty has enough information to create an offer. If they send their preparedkeysfile to you, you can create the offer. This has a split, which you might need to discuss with your peer. Either way, the command to advance to the next step is:
```
chantools zombierecovery makeoffer \
	--node1_keys preparedkeys-xxxx-xx-xx-<pubkey1>.json \
	--node2_keys preparedkeys-xxxx-xx-xx-<pubkey2>.json \
	--feerate 15
```
7. The output is a PSBT. It's signed by the party which created the offer. This must now be signed by the other party (thereby accepting the offer):
```
chantools zombierecovery signoffer \
	--psbt <offered_psbt_base64>
```
9. After signing, the transaction can be broadcast.

## File format
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
            "short_channel_id": "61xxxxxxxxxxxxx (numerical channel ID, can be found on 1ml.com)",
            "chan_point": "<txid>:<output_index> (also called channel point on 1ml.com)",
            "address": "bc1q...... (the channel's output address on chain, find out by looking up the channel point on a block explorer)",
            "capacity": 123456
        }
    ]
}
```

## More info
_More info at the help output of `chantools zombierecovery --help` or the generated [documentation for the zombierecovery command](chantools_zombierecovery.md)._
