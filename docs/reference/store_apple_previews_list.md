## store apple previews list

List the app preview sets of a version

### Synopsis

Lists the app preview sets per locale and device with their previews in order. Without --version:
the editable version if there is one, else the live one. Result: [PreviewSet].

```
store apple previews list [--locale <l>] [--device <d>] [--version <v>] [flags]
```

### Examples

```
  store apple previews list --locale en-US --device iphone-6.9
```

### Options

```
      --device string    only this device, e.g. iphone-6.9
  -h, --help             help for list
      --locale string    only this locale
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

