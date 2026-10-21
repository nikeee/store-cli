## store apple version phased start

Enable phased release for a version

### Synopsis

Enables phased release for a version that is not live yet. It starts with the release. For a version
that is live it starts at once. A version that has a phased release is left as it is.
Result: {version, state, currentDayNumber}.

```
store apple version phased start <version> [flags]
```

### Examples

```
  store apple version phased start 1.4.0
```

### Options

```
      --dry-run   print what would change, change nothing
  -h, --help      help for start
```

### Options inherited from parent commands

```
      --app string        bundle ID of the app, e.g. com.example.app (default $APPLE_APPSTORE_BUNDLE_ID)
  -o, --output string     output format: text, json or toon (default: auto)
      --platform string   platform of versions, builds and screenshots: ios, macos, tvos or visionos (default "ios")
  -v, --verbose           log every HTTP request to stderr
```

### SEE ALSO

* [store apple version phased](store_apple_version_phased.md)	 - Manage the phased release of a version

