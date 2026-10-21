## store apple listing show

Show the listing texts per locale

### Synopsis

Shows the app info and version texts per locale. Without --version: the editable version if there
is one, else the live one.

Result: [Localization]

```
store apple listing show [flags]
```

### Examples

```
  store apple listing show --locale de-DE
```

### Options

```
  -h, --help             help for show
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

* [store apple listing](store_apple_listing.md)	 - Show and change the store listing texts

