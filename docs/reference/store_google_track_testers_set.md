## store google track testers set

Replace the tester groups of a track

### Synopsis

Replaces the Google Groups that test a track with the given --group addresses (repeatable, at
least one). With --dry-run the result shows what would be stored.

Result: Testers

```
store google track testers set <track> --group <google-group-email>... [flags]
```

### Examples

```
  store google track testers set internal --group qa@googlegroups.com --group beta@example.com
```

### Options

```
      --changes-not-sent-for-review   commit without sending the changes for review; needed when Play Console requires sending them manually
      --dry-run                       print what would change, change nothing
      --group stringArray             Google Group email address of the testers (repeatable)
  -h, --help                          help for set
```

### Options inherited from parent commands

```
      --app string      package name, e.g. com.example.app (default $GOOGLE_PLAY_PACKAGE_NAME)
  -o, --output string   output format: text, json or toon (default: auto)
  -v, --verbose         log every HTTP request to stderr
```

### SEE ALSO

* [store google track testers](store_google_track_testers.md)	 - Show or set the tester groups of a track

