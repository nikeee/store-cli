## store apple review-info show

Show the App Review information

### Synopsis

Shows the App Review contact, demo account, notes and attachments of a version. Without --version:
the editable version if there is one, else the live one. The demo password is shown as *** unless
--reveal is given. Result: {firstName, lastName, phone, email, demoRequired, demoUser,
demoPassword, notes, attachments: [fileName]}.

```
store apple review-info show [--version <v>] [flags]
```

### Examples

```
  store apple review-info show --reveal
```

### Options

```
  -h, --help             help for show
      --reveal           show the demo password instead of ***
      --version string   app version (default: the editable version, else the live one)
```

### Options inherited from parent commands

```
      --app string        bundle ID of the app, e.g. com.example.app (default $APPLE_APPSTORE_BUNDLE_ID)
  -o, --output string     output format: text, json or toon (default: auto)
      --platform string   platform of versions, builds and screenshots: ios, macos, tvos or visionos (default "ios")
  -v, --verbose           log every HTTP request to stderr
```

### SEE ALSO

* [store apple review-info](store_apple_review-info.md)	 - Show or change the App Review information

