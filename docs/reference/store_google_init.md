## store google init

Scaffold the Play listing directory for a new app

### Synopsis

Scaffolds <dir>/android/<locale>/ per locale: one empty file per row of the field table and the
images/ folders of the image type table. Same rules as store apple init: never touches existing
files, so it can be re-run to add a locale; works offline, needs no credentials. For an app that
already has a listing, use pull instead (an empty file clears its field on push). --locale takes
an App Store style locale or a Play code; locales are written as Play codes.
Text mode also prints each created file with its character limit.

Result: InitResult

```
store google init <dir> --locale <l> [--locale <l>]... [flags]
```

### Examples

```
  store google init fastlane/metadata --locale en-US --locale de-DE
```

### Options

```
  -h, --help                 help for init
      --locale stringArray   locale to scaffold, e.g. en-US (required, repeatable)
```

### Options inherited from parent commands

```
      --app string      package name, e.g. com.example.app (default $GOOGLE_PLAY_PACKAGE_NAME)
  -o, --output string   output format: text, json or toon (default: auto)
  -v, --verbose         log every HTTP request to stderr
```

### SEE ALSO

* [store google](store_google.md)	 - Manage apps in Google Play

