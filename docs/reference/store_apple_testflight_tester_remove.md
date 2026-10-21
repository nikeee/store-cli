## store apple testflight tester remove

Remove a tester from a beta group

### Synopsis

Removes the tester from the group only. Result: the tester.

```
store apple testflight tester remove <email> [flags]
```

### Examples

```
  store apple testflight tester remove ada@example.com --group Beta
```

### Options

```
      --dry-run        print what would change, change nothing
      --group string   beta group (name or ID)
  -h, --help           help for remove
```

### Options inherited from parent commands

```
      --app string        bundle ID of the app, e.g. com.example.app (default $APPLE_APPSTORE_BUNDLE_ID)
  -o, --output string     output format: text, json or toon (default: auto)
      --platform string   platform of versions, builds and screenshots: ios, macos, tvos or visionos (default "ios")
  -v, --verbose           log every HTTP request to stderr
```

### SEE ALSO

* [store apple testflight tester](store_apple_testflight_tester.md)	 - Manage TestFlight testers

