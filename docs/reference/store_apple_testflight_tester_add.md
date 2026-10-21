## store apple testflight tester add

Add a tester to a beta group

### Synopsis

Creates the tester if needed and adds them to the group; already a member is a no-op.

Result: Tester

```
store apple testflight tester add <email> [flags]
```

### Examples

```
  store apple testflight tester add ada@example.com --group Beta --first-name Ada
```

### Options

```
      --dry-run             print what would change, change nothing
      --first-name string   first name of a new tester
      --group string        beta group (name or ID)
  -h, --help                help for add
      --last-name string    last name of a new tester
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

