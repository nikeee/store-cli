## store apple testflight submit

Submit a build to Beta App Review

### Synopsis

Submits the build to Beta App Review (needed for external testing). Already submitted is a no-op.
Result: {build, state}.

```
store apple testflight submit <build> [flags]
```

### Examples

```
  store apple testflight submit latest
```

### Options

```
      --dry-run          print what would change, change nothing
  -h, --help             help for submit
      --version string   app version the build belongs to (needed when the build number exists under several versions)
```

### Options inherited from parent commands

```
      --app string        bundle ID of the app, e.g. com.example.app (default $APPLE_APPSTORE_BUNDLE_ID)
  -o, --output string     output format: text, json or toon (default: auto)
      --platform string   platform of versions, builds and screenshots: ios, macos, tvos or visionos (default "ios")
  -v, --verbose           log every HTTP request to stderr
```

### SEE ALSO

* [store apple testflight](store_apple_testflight.md)	 - Manage TestFlight

