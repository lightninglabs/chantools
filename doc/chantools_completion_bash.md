## chantools completion bash

Generate the autocompletion script for bash

### Synopsis

Generate the autocompletion script for the bash shell.

This script depends on the 'bash-completion' package.
If it is not installed already, you can install it via your OS's package manager.

To load completions in your current shell session:

	source <(chantools completion bash)

To load completions for every new session, execute once:

#### Linux:

	chantools completion bash > /etc/bash_completion.d/chantools

#### macOS:

	chantools completion bash > $(brew --prefix)/etc/bash_completion.d/chantools

You will need to start a new shell for this setup to take effect.


```
chantools completion bash
```

### Options

```
  -h, --help              help for bash
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

