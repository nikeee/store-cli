## store apple version create

Create a new App Store version

### Synopsis

Creates the version. Fails if the version exists. --release scheduled needs --release-date.

Result: Version

```
store apple version create <version> [flags]
```

### Examples

```
  store apple version create 1.4.0 --release scheduled --release-date 2026-12-01T09:00:00Z
```

### Options

```
      --copyright string      copyright text, or @file to read it from a file
      --dry-run               print what would change, change nothing
  -h, --help                  help for create
      --release string        how the version is released: manual, after-approval or scheduled
      --release-date string   release time for --release scheduled, an RFC 3339 time
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

