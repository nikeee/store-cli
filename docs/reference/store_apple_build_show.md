## store apple build show

Show one build

### Synopsis

Result: Build.

```
store apple build show <build> [flags]
```

### Examples

```
  store apple build show 45 --version 1.4.0
```

### Options

```
  -h, --help             help for show
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

* [store apple build](store_apple_build.md)	 - List, inspect and update builds

