## store apple listing set

Change listing texts of one locale

### Synopsis

Changes the given fields of one locale; fields not given stay as they are. A locale that does not
exist yet is created. An empty value clears a field. Lengths are checked before any API call.
App info fields (name, subtitle, privacy URLs) need an editable app info. A text starting with @
is read from that file. With --dry-run the result is the planned Plan.

Result: Localization

```
store apple listing set --locale <l> [--version <v>] [field flags] [flags]
```

### Examples

```
  store apple listing set --locale en-US --subtitle "Fast and small" --keywords @keywords.txt
```

### Options

```
      --description string           App description, or @file to read it from a file
      --dry-run                      print what would change, change nothing
  -h, --help                         help for set
      --keywords string              Search keywords, comma separated, or @file to read it from a file
      --locale string                locale to change, e.g. en-US (required)
      --marketing-url string         Marketing URL, or @file to read it from a file
      --name string                  App name, or @file to read it from a file
      --privacy-choices-url string   Privacy choices URL, or @file to read it from a file
      --privacy-url string           Privacy policy URL, or @file to read it from a file
      --promotional-text string      Promotional text, or @file to read it from a file
      --subtitle string              App subtitle, or @file to read it from a file
      --support-url string           Support URL, or @file to read it from a file
      --version string               app version (default: the editable version, else the live one)
```

### Options inherited from parent commands

```
      --app string        bundle ID of the app, e.g. com.example.app (default $APPLE_APPSTORE_BUNDLE_ID)
  -o, --output string     output format: text, json or toon (default: auto)
      --platform string   platform of versions, builds and screenshots: ios, macos, tvos or visionos (default "ios")
  -v, --verbose           log every HTTP request to stderr
```

### SEE ALSO

* [store apple listing](store_apple_listing.md)	 - Show and change the store listing texts

