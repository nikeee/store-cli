## store apple testflight group create

Create a beta group

### Synopsis

Creates a beta group, optionally internal or with a public link.

Result: BetaGroup

```
store apple testflight group create <name> [flags]
```

### Examples

```
  store apple testflight group create Friends --public-link --public-link-limit 100
```

### Options

```
      --dry-run                 print what would change, change nothing
  -h, --help                    help for create
      --internal                create an internal group
      --public-link             enable the public link
      --public-link-limit int   maximum number of testers joining through the public link
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

