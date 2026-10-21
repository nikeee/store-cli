## store apple testflight group list

List the beta groups of the app

### Synopsis

Lists the beta groups of the app.

Result: [BetaGroup]

```
store apple testflight group list [flags]
```

### Examples

```
  store apple testflight group list
```

### Options

```
  -h, --help   help for list
```

### Options inherited from parent commands

```
      --app string        bundle ID of the app, e.g. com.example.app (default $APPLE_APPSTORE_BUNDLE_ID)
  -o, --output string     output format: text, json or toon (default: auto)
      --platform string   platform of versions, builds and screenshots: ios, macos, tvos or visionos (default "ios")
  -v, --verbose           log every HTTP request to stderr
```

### SEE ALSO

* [store apple testflight group](store_apple_testflight_group.md)	 - Manage TestFlight beta groups

