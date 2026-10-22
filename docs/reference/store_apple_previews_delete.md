## store apple previews delete

Delete the app preview set of one locale and device

### Synopsis

Deletes the whole set. A set that does not exist is a skip (exit 0). With --dry-run the result
is the planned Plan.

Result: PreviewsDeleted

```
store apple previews delete --locale <l> --device <d> [--version <v>] [flags]
```

### Examples

```
  store apple previews delete --locale en-US --device iphone-6.5
```

### Options

```
      --device string    device of the set, e.g. iphone-6.9 (required)
      --dry-run          print what would change, change nothing
  -h, --help             help for delete
      --locale string    locale of the set, e.g. en-US (required)
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

* [store apple previews](store_apple_previews.md)	 - List, upload and delete app previews

