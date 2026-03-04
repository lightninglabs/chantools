## chantools completion fish

Generate the autocompletion script for fish

### Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	chantools completion fish | source

To load completions for every new session, execute once:

	chantools completion fish > ~/.config/fish/completions/chantools.fish

You will need to start a new shell for this setup to take effect.


```
chantools completion fish [flags]
```

### Options

```
  -h, --help              help for fish
      --no-descriptions   disable completion descriptions
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

### SEE ALSO

* [chantools completion](chantools_completion.md)	 - Generate the autocompletion script for the specified shell

