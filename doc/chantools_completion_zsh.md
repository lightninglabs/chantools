## chantools completion zsh

Generate the autocompletion script for zsh

### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(chantools completion zsh)

To load completions for every new session, execute once:

#### Linux:

	chantools completion zsh > "${fpath[1]}/_chantools"

#### macOS:

	chantools completion zsh > $(brew --prefix)/share/zsh/site-functions/_chantools

You will need to start a new shell for this setup to take effect.


```
chantools completion zsh [flags]
```

### Options

```
  -h, --help              help for zsh
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

