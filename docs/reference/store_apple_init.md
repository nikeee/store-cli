## store apple init

Scaffold the metadata directory for a new app

### Synopsis

Scaffolds the metadata directory for a new app: per locale one empty file per row of the field
table, review_information/ with its files, and <screenshots>/<locale>/. Never touches existing
files, so it can be re-run to add a locale. Works offline, needs no credentials. For an app that
already has a listing, use pull instead (an empty file clears its field on push).
Result: {created: [path], existing: [path]}. Text mode also prints each created file with its
character limit.

```
store apple init <dir> --locale <l> [--locale <l>]... [--screenshots <dir>] [flags]
```

### Examples

```
  store apple init fastlane/metadata --locale en-US --locale de-DE
```

### Options

```
  -h, --help                 help for init
      --locale stringArray   locale to scaffold, e.g. en-US (required, repeatable)
      --screenshots string   screenshots directory (default: <dir>/../screenshots)
```

### Options inherited from parent commands

```
      --app string        bundle ID of the app, e.g. com.example.app (default $APPLE_APPSTORE_BUNDLE_ID)
  -o, --output string     output format: text, json or toon (default: auto)
      --platform string   platform of versions, builds and screenshots: ios, macos, tvos or visionos (default "ios")
  -v, --verbose           log every HTTP request to stderr
```

### SEE ALSO

* [store apple](store_apple.md)	 - Manage apps in App Store Connect

