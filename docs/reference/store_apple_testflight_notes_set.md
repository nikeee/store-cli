## store apple testflight notes set

Set the What to Test text of a build

### Synopsis

Sets the "What to Test" text (max 4000 characters). Works while the build is still processing.
A text starting with @ is read from that file. A build appears in App Store Connect only some
minutes after its upload: --wait-for-build waits up to that long for it instead of failing.

Result: BuildNotes

```
store apple testflight notes set <build> [flags]
```

### Examples

```
  store apple testflight notes set 45 --text "Fixed login"
  store apple testflight notes set latest --text @notes.txt --locale de-DE
  store apple testflight notes set 46 --text @notes.txt --wait-for-build 15m
```

### Options

```
      --dry-run                   print what would change, change nothing
  -h, --help                      help for set
      --locale string             locale of the text (default "en-US")
      --text string               the What to Test text, or @file to read it from a file (max 4000 characters)
      --version string            app version the build belongs to (needed when the build number exists under several versions)
      --wait-for-build duration   wait up to this long for the build to appear, e.g. 15m
```

### Options inherited from parent commands

```
      --app string        bundle ID of the app, e.g. com.example.app (default $APPLE_APPSTORE_BUNDLE_ID)
  -o, --output string     output format: text, json or toon (default: auto)
      --platform string   platform of versions, builds and screenshots: ios, macos, tvos or visionos (default "ios")
  -v, --verbose           log every HTTP request to stderr
```

### SEE ALSO

* [store apple testflight notes](store_apple_testflight_notes.md)	 - Manage the What to Test notes of builds

