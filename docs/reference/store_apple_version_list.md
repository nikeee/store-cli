## store apple version list

List the versions of the app, newest first

### Synopsis

Lists the versions of the app on --platform, newest first.

Result: [Version]

```
store apple version list [flags]
```

### Examples

```
  store apple version list --limit 5
```

### Options

```
  -h, --help        help for list
      --limit int   maximum number of versions (default 10)
```

### Options inherited from parent commands

```
      --app string        bundle ID of the app, e.g. com.example.app (default $APPLE_APPSTORE_BUNDLE_ID)
  -o, --output string     output format: text, json or toon (default: auto)
      --platform string   platform of versions, builds and screenshots: ios, macos, tvos or visionos (default "ios")
  -v, --verbose           log every HTTP request to stderr
```

### SEE ALSO

* [store apple version](store_apple_version.md)	 - Manage App Store versions

