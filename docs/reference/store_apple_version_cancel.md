## store apple version cancel

Withdraw the review submission of a version

### Synopsis

Withdraws the version's review submission. Result: {version, submissionId, state}.

```
store apple version cancel <version> [flags]
```

### Examples

```
  store apple version cancel 1.4.0
```

### Options

```
      --dry-run   print what would change, change nothing
  -h, --help      help for cancel
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

