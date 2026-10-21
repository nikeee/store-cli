## store apple build list

List builds, newest first

### Synopsis

Lists the builds of the app, newest first.

Result: [Build]

```
store apple build list [flags]
```

### Examples

```
  store apple build list --version 1.4.0 --state valid --limit 5
```

### Options

```
  -h, --help             help for list
      --limit int        maximum number of builds (default 20)
      --state string     only builds in this processing state: processing, failed, invalid or valid
      --version string   only builds of this app version
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

