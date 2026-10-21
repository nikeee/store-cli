## store apple app show

Show the app given by --app

### Synopsis

The app given by --app. Result: {app: App, versions: [Version], locales: [string]}.
versions holds the newest versions per platform (at most 5).

```
store apple app show [flags]
```

### Examples

```
  store apple app show --app com.example.app
```

### Options

```
  -h, --help   help for show
```

### Options inherited from parent commands

```
      --app string        bundle ID of the app, e.g. com.example.app (default $APPLE_APPSTORE_BUNDLE_ID)
  -o, --output string     output format: text, json or toon (default: auto)
      --platform string   platform of versions, builds and screenshots: ios, macos, tvos or visionos (default "ios")
  -v, --verbose           log every HTTP request to stderr
```

### SEE ALSO

* [store apple app](store_apple_app.md)	 - Show the apps of the team

