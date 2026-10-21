## store apple version submit

Submit a version for App Review

### Synopsis

Submits the version for App Review. An open, unsubmitted review submission of the app is reused
together with its other items (Apple only accepts e.g. a first subscription group together with an
app version). If that open submission holds a different version, the command fails and names it.
Result: {version, submissionId, state, items}.

```
store apple version submit <version> [flags]
```

### Examples

```
  store apple version submit 1.4.0
```

### Options

```
      --dry-run   print what would change, change nothing
  -h, --help      help for submit
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

