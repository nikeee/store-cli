## store google track promote

Copy a release from one track to another

### Synopsis

Copies a release (by name, default the newest by version code) from --from to --to. The release
becomes completed on the target, or in-progress with --rollout. Release notes come from
--release-notes, else from the source release; languages the listing does not have are dropped
with a warning. A release that is already completed on the target is a no-op. Without a match,
the error lists the release names of the source track. --status in-progress and halted need
--rollout. Result: Track (the target).

```
store google track promote [<release-name>] --from <track> --to <track> [flags]
```

### Examples

```
  store google track promote --from internal --to production --rollout 0.1
  store google track promote v1.4.0 --from beta --to production
```

### Options

```
      --changes-not-sent-for-review   commit without sending the changes for review; needed when Play Console requires sending them manually
      --dry-run                       print what would change, change nothing
      --from string                   source track
  -h, --help                          help for promote
      --release-notes string          directory with one <locale>.txt per locale; default: the notes of the source release
      --rollout float                 staged rollout fraction, greater than 0 and less than 1; implies --status in-progress
      --status string                 target release status: completed, draft, in-progress or halted (default "completed")
      --to string                     target track
```

### Options inherited from parent commands

```
      --app string      package name, e.g. com.example.app (default $GOOGLE_PLAY_PACKAGE_NAME)
  -o, --output string   output format: text, json or toon (default: auto)
  -v, --verbose         log every HTTP request to stderr
```

### SEE ALSO

* [store google track](store_google_track.md)	 - Inspect and promote release tracks

