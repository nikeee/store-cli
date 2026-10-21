## store google track rollout

Change the rollout fraction of a track

### Synopsis

Sets the user fraction of the track's staged (in progress) release. The fraction must be greater
than 0 and less than 1; use `track complete` to reach everyone. A track without a staged release
is an error. Result: Track.

```
store google track rollout <track> <fraction> [flags]
```

### Examples

```
  store google track rollout production 0.5
```

### Options

```
      --changes-not-sent-for-review   commit without sending the changes for review; needed when Play Console requires sending them manually
      --dry-run                       print what would change, change nothing
  -h, --help                          help for rollout
```

### Options inherited from parent commands

```
      --app string      package name, e.g. com.example.app (default $GOOGLE_PLAY_PACKAGE_NAME)
  -o, --output string   output format: text, json or toon (default: auto)
  -v, --verbose         log every HTTP request to stderr
```

### SEE ALSO

* [store google track](store_google_track.md)	 - Inspect and promote release tracks

