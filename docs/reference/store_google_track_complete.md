## store google track complete

Roll the staged release of a track out to everyone

### Synopsis

Completes the track's staged or halted release (status completed, no user fraction). It replaces
the other releases of the track.

Result: Track

```
store google track complete <track> [flags]
```

### Examples

```
  store google track complete production
```

### Options

```
      --changes-not-sent-for-review   commit without sending the changes for review; needed when Play Console requires sending them manually
      --dry-run                       print what would change, change nothing
  -h, --help                          help for complete
```

### Options inherited from parent commands

```
      --app string      package name, e.g. com.example.app (default $GOOGLE_PLAY_PACKAGE_NAME)
  -o, --output string   output format: text, json or toon (default: auto)
  -v, --verbose         log every HTTP request to stderr
```

### SEE ALSO

* [store google track](store_google_track.md)	 - Inspect and promote release tracks

