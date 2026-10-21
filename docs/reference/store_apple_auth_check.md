## store apple auth check

Verify the credentials with one read call

### Synopsis

Verifies the credentials with one read call. Never prints the key.

Result: AppleAuthCheck

```
store apple auth check [flags]
```

### Examples

```
  store apple auth check
```

### Options

```
  -h, --help   help for check
```

### Options inherited from parent commands

```
      --app string        bundle ID of the app, e.g. com.example.app (default $APPLE_APPSTORE_BUNDLE_ID)
  -o, --output string     output format: text, json or toon (default: auto)
      --platform string   platform of versions, builds and screenshots: ios, macos, tvos or visionos (default "ios")
  -v, --verbose           log every HTTP request to stderr
```

### SEE ALSO

* [store apple auth](store_apple_auth.md)	 - Check the App Store Connect credentials

