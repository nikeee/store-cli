## store google release-notes set

Set the release notes of a release on a track

### Synopsis

Sets the release notes of a release on a track, by --release-name or, by default, the newest
release by version code. --dir reads <locale>.txt per locale. The notes of other languages stay.
Max 500 characters per locale; locales the listing does not have are dropped with a warning.
The other releases of the track are not changed. Result: Release.

```
store google release-notes set <track> (--dir <dir> | --locale <l> --text <text>) [flags]
```

### Examples

```
  store google release-notes set production --locale en-US --text "Bug fixes"
  store google release-notes set internal --release-name v1.4.0 --dir release-notes
```

### Options

```
      --changes-not-sent-for-review   commit without sending the changes for review; needed when Play Console requires sending them manually
      --dir string                    directory with one <locale>.txt per locale
      --dry-run                       print what would change, change nothing
  -h, --help                          help for set
      --locale string                 locale of --text
      --release-name string           release to change (default: the newest by version code)
      --text string                   the release notes, or @file to read them from a file (max 500 characters)
```

### Options inherited from parent commands

```
      --app string      package name, e.g. com.example.app (default $GOOGLE_PLAY_PACKAGE_NAME)
  -o, --output string   output format: text, json or toon (default: auto)
  -v, --verbose         log every HTTP request to stderr
```

### SEE ALSO

* [store google release-notes](store_google_release-notes.md)	 - Set the release notes of a release

