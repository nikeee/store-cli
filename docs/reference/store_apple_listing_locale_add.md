## store apple listing locale add

Add a locale to the version and the app info

### Synopsis

Creates the localization on the version and the app info where it is missing. Result: {locale, action}.

```
store apple listing locale add <locale> [flags]
```

### Examples

```
  store apple listing locale add de-DE
```

### Options

```
      --dry-run          print what would change, change nothing
  -h, --help             help for add
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

