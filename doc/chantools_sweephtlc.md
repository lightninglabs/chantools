## chantools sweephtlc

Sweep channel HTLC outputs by matching exact outpoints against `channel.db`.

### Synopsis

This command takes one or more on-chain HTLC output outpoints, loads the
corresponding channel from an lnd `channel.db`, reconstructs candidate HTLC
scripts, and signs the matching spend.

The first supported spend path is an outgoing HTLC on the remote party's
commitment transaction after CLTV timeout. This is the direct timeout spend used
when the remote party force-closes with an HTLC we offered.

By default the command only prints the raw transaction. Use `--publish` to
publish through the configured Esplora-compatible API.

```
chantools sweephtlc [flags]
```

### Examples

```
chantools sweephtlc \
  --channeldb ~/.lnd/data/graph/mainnet/channel.db \
  --outpoints aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:3,aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa:4 \
  --commitpoint 0279be667ef9dcbbac55a06295ce870b07029bfcdb2dce28d959f2815b16f81798 \
  --sweepaddr bc1q..... \
  --feerate 1
```

### Options

```
      --apiurl string        API URL to use (must be esplora compatible) (default "https://api.node-recovery.com")
      --bip39                read a classic BIP39 seed and passphrase from the terminal instead of asking for lnd seed format or providing the --rootkey flag
      --channeldb string     lnd channel.db file to read channel state from
      --commitpoint string   optional commitment point override to try when reconstructing HTLC scripts
      --feerate uint32       fee rate to use for the sweep transaction in sat/vByte (default 1)
  -h, --help                 help for sweephtlc
      --outpoints string     comma separated HTLC outpoints to sweep, in txid:index format
      --publish              publish sweep TX to the chain API instead of just printing the TX
      --rootkey string       BIP32 HD root key of the wallet to use for signing HTLC sweep transaction; leave empty to prompt for lnd 24 word aezeed
      --sweepaddr string     address to recover the funds to; specify 'fromseed' to derive a new address from the seed automatically
      --walletdb string      read the seed/master root key to use for signing HTLC sweep transaction from an lnd wallet.db file instead of asking for a seed or providing the --rootkey flag
```

### Options inherited from parent commands

```
      --nologfile           If set, no log file will be created. This is useful for testing purposes where we don't want to create a log file.
  -r, --regtest             Indicates if regtest parameters should be used
      --resultsdir string   Directory where results should be stored (default "./results")
  -s, --signet              Indicates if the public signet parameters should be used
  -t, --testnet             Indicates if testnet parameters should be used
      --testnet4            Indicates if testnet4 parameters should be used
```

### Notes

- Supported in v1: segwit v0 outgoing HTLCs on a remote commitment after CLTV timeout.
- Unsupported in v1: incoming/preimage HTLCs, local commitment second-level HTLC flows, taproot HTLCs.
- `--commitpoint` is useful for data-loss-protection cases where `channel.db` has the HTLC metadata but the remote force-close commitment point must be supplied from the DLP message.
- `sweephtlc` cannot recover HTLC outputs whose payment hash, CLTV expiry, amount, and direction are absent from `channel.db`.

### SEE ALSO

* [chantools](chantools.md) - Chantools helps recover funds from lightning channels
