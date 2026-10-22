## store apple pull

Write the store listing into a metadata directory

### Synopsis

Writes the text files of every store locale and review_information/ (without the demo password)
into <dir>. A field without a value in the store is written as an empty file, so a value cleared
in the store stays cleared on the next push. Overwrites the files it writes and touches nothing
else. Does not download screenshots (Apple re-encodes them, so they would never match the
originals). Without --version: the editable version if there is one, else the live one.
A store locale that is not a locale name is skipped with a warning.

Result: ApplePullResult

```
store apple pull <dir> [--version <v>] [flags]
```

### Examples

```
  store apple pull fastlane/metadata
```

### Options

```
  -h, --help             help for pull
      --version string   app version (default: the editable version, else the live one)
```

### Options inherited from parent commands

```
      --app string        bundle ID of the app, e.g. com.example.app (default $APPLE_APPSTORE_BUNDLE_ID)
  -o, --output string     output format: text, json or toon (default: auto)
      --platform string   platform of versions, builds and screenshots: ios, macos, tvos or visionos (default "ios")
  -v, --verbose           log every HTTP request to stderr
```

### SEE ALSO

* [store apple](store_apple.md)	 - Manage apps in App Store Connect

