## store apple release-notes set

Set What's New of a version

### Synopsis

Sets "What's New". --dir reads <locale>.txt per locale. A locale without a localization on the
version is an error that suggests `listing locale add`. App Store Connect accepts What's New only
after the first release.

Result: [WhatsNew]

```
store apple release-notes set <version> (--dir <dir> | --locale <l> --text <text>) [flags]
```

### Examples

```
  store apple release-notes set 1.4.0 --locale en-US --text "Bug fixes"
  store apple release-notes set 1.4.0 --dir release-notes
```

### Options

```
      --dir string      directory with one <locale>.txt per locale
      --dry-run         print what would change, change nothing
  -h, --help            help for set
      --locale string   locale of --text
      --text string     the What's New text, or @file to read it from a file (max 4000 characters)
```

### Options inherited from parent commands

```
      --app string        bundle ID of the app, e.g. com.example.app (default $APPLE_APPSTORE_BUNDLE_ID)
  -o, --output string     output format: text, json or toon (default: auto)
      --platform string   platform of versions, builds and screenshots: ios, macos, tvos or visionos (default "ios")
  -v, --verbose           log every HTTP request to stderr
```

### SEE ALSO

* [store apple release-notes](store_apple_release-notes.md)	 - Set the What's New text

