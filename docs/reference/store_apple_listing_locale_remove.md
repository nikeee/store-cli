## store apple listing locale remove

Remove a locale from the version and the app info

### Synopsis

Deletes the localization from the version and the app info. The primary locale cannot be removed.

Result: LocaleResult

```
store apple listing locale remove <locale> [flags]
```

### Examples

```
  store apple listing locale remove de-DE
```

### Options

```
      --dry-run          print what would change, change nothing
  -h, --help             help for remove
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

* [store apple listing locale](store_apple_listing_locale.md)	 - Add or remove a locale

