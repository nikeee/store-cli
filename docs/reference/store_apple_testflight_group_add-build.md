## store apple testflight group add-build

Add a build to a beta group

### Synopsis

Adds a build (number or latest) to a beta group, given by name or ID. External groups need a build
that passed Beta App Review (see testflight submit). Result: {group, build}.

```
store apple testflight group add-build <group> <build> [flags]
```

### Examples

```
  store apple testflight group add-build Beta latest
```

### Options

```
      --dry-run          print what would change, change nothing
  -h, --help             help for add-build
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

* [store apple testflight group](store_apple_testflight_group.md)	 - Manage TestFlight beta groups

