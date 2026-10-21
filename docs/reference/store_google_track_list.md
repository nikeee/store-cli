## store google track list

List the tracks and their releases

### Synopsis

Lists the tracks: internal, alpha, beta, production and custom closed testing tracks, with their
releases.

Result: [Track]

```
store google track list [flags]
```

### Examples

```
  store google track list
```

### Options

```
  -h, --help   help for list
```

### Options inherited from parent commands

```
      --app string      package name, e.g. com.example.app (default $GOOGLE_PLAY_PACKAGE_NAME)
  -o, --output string   output format: text, json or toon (default: auto)
  -v, --verbose         log every HTTP request to stderr
```

### SEE ALSO

* [store google track](store_google_track.md)	 - Inspect and promote release tracks

