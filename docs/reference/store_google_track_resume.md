## store google track resume

Resume the halted rollout of a track

### Synopsis

Resumes the track's halted release (status in progress, user fraction kept).

Result: Track

```
store google track resume <track> [flags]
```

### Examples

```
  store google track resume production
```

### Options

```
      --changes-not-sent-for-review   commit without sending the changes for review; needed when Play Console requires sending them manually
      --dry-run                       print what would change, change nothing
  -h, --help                          help for resume
```

### Options inherited from parent commands

```
      --app string      package name, e.g. com.example.app (default $GOOGLE_PLAY_PACKAGE_NAME)
  -o, --output string   output format: text, json or toon (default: auto)
  -v, --verbose         log every HTTP request to stderr
```

### SEE ALSO

* [store google track](store_google_track.md)	 - Inspect and promote release tracks

