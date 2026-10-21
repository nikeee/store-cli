## store apple version phased complete

Release to all users now, ending the phased release

### Synopsis

Sets the phased release of the version to COMPLETE. Result: {version, state, currentDayNumber}.

```
store apple version phased complete <version> [flags]
```

### Examples

```
  store apple version phased complete 1.4.0
```

### Options

```
      --dry-run   print what would change, change nothing
  -h, --help      help for complete
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

