## store apple version set-build

Attach a build to an editable version

### Synopsis

Attaches a processed (VALID) build to an editable version. The build is looked up under the
version's own number.

Result: Version

```
store apple version set-build <version> <build> [flags]
```

### Examples

```
  store apple version set-build 1.4.0 45
  store apple version set-build 1.4.0 latest
```

### Options

```
      --dry-run   print what would change, change nothing
  -h, --help      help for set-build
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

