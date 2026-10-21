## store apple testflight tester list

List the testers of the app

### Synopsis

Lists the testers of the app, optionally only those of one group. Result: [{id, email, firstName, lastName, inviteType, state}].

```
store apple testflight tester list [flags]
```

### Examples

```
  store apple testflight tester list --group Beta
```

### Options

```
      --group string   only testers of this group (name or ID)
  -h, --help           help for list
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

