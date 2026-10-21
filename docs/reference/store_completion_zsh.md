## store completion zsh

Generate the autocompletion script for zsh

### Synopsis

Generate the autocompletion script for the zsh shell.

If shell completion is not already enabled in your environment you will need
to enable it.  You can execute the following once:

	echo "autoload -U compinit; compinit" >> ~/.zshrc

To load completions in your current shell session:

	source <(store completion zsh)

To load completions for every new session, execute once:

#### Linux:

	store completion zsh > "${fpath[1]}/_store"

#### macOS:

	store completion zsh > $(brew --prefix)/share/zsh/site-functions/_store

You will need to start a new shell for this setup to take effect.


```
store completion zsh [flags]
```

### Options

```
  -h, --help              help for zsh
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json or toon (default: auto)
  -v, --verbose         log every HTTP request to stderr
```

### SEE ALSO

* [store completion](store_completion.md)	 - Generate the autocompletion script for the specified shell

