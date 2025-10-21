## chantools vanitygen

Generate a seed with a custom lnd node identity public key that starts with the given prefix

### Synopsis

Try random lnd compatible seeds until one is found that
produces a node identity public key that starts with the given prefix.

Example output:

<pre>
Running vanitygen on 8 threads. Prefix bit length is 17, expecting to approach
probability p=1.0 after 131,072 seeds.
Tested 185k seeds, p=1.41296, speed=14k/s, elapsed=13s                          
Looking for 022222, found pubkey: 022222f015540ddde9bdf7c95b24f1d44f7ea6ab69bec83d6fbe622296d64b51d6
with seed: [ability roast pear stomach wink cable tube trumpet shy caught hunt
someone border organ spoon only prepare calm silent million tobacco chaos normal
phone]
</pre>


```
chantools vanitygen [flags]
```

### Examples

```
chantools vanitygen --prefix 022222 --threads 8
```

### Options

```
  -h, --help            help for vanitygen
      --prefix string   hex encoded prefix to find in node public key
      --threads uint8   number of parallel threads (default 4)
```

### Options inherited from parent commands

```
      --nologfile           If set, no log file will be created. This is useful for testing purposes where we don't want to create a log file.
  -r, --regtest             Indicates if regtest parameters should be used
      --resultsdir string   Directory where results should be stored (default "./results")
  -s, --signet              Indicates if the public signet parameters should be used
  -t, --testnet             Indicates if testnet parameters should be used
```

### SEE ALSO

* [chantools](chantools.md)	 - Chantools helps recover funds from lightning channels

