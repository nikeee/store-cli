## store apple build wait

Wait until a build exists and is processed

### Synopsis

Waits until the build exists and is processed. VALID exits 0; FAILED or INVALID exits 1.

Result: Build

```
store apple build wait <build> [flags]
```

### Examples

```
  store apple build wait 45 --timeout 20m
```

### Options

```
  -h, --help               help for wait
      --timeout duration   how long to wait, e.g. 90s, 10m, 1h (default 30m0s)
      --version string     app version the build belongs to (needed when the build number exists under several versions)
```

### Options inherited from parent commands

```
      --app string        bundle ID of the app, e.g. com.example.app (default $APPLE_APPSTORE_BUNDLE_ID)
  -o, --output string     output format: text, json or toon (default: auto)
      --platform string   platform of versions, builds and screenshots: ios, macos, tvos or visionos (default "ios")
  -v, --verbose           log every HTTP request to stderr
```

### SEE ALSO

* [store apple build](store_apple_build.md)	 - List, inspect and update builds

