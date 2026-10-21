## store completion fish

Generate the autocompletion script for fish

### Synopsis

Generate the autocompletion script for the fish shell.

To load completions in your current shell session:

	store completion fish | source

To load completions for every new session, execute once:

	store completion fish > ~/.config/fish/completions/store.fish

You will need to start a new shell for this setup to take effect.


```
store completion fish [flags]
```

### Options

```
  -h, --help              help for fish
      --no-descriptions   disable completion descriptions
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json or toon (default: auto)
  -v, --verbose         log every HTTP request to stderr
```

### SEE ALSO

* [store completion](store_completion.md)	 - Generate the autocompletion script for the specified shell

