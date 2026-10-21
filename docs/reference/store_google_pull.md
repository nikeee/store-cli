## store google pull

Write the Play listing into a metadata directory

### Synopsis

Writes the text files of every Play listing locale into <dir>/android/<locale>/ (Play codes). A
field without a value in Play is written as an empty file, so a value cleared in Play stays cleared
on the next push. Overwrites the files it writes and touches nothing else. No images in v1.
Result: {locales: [string], files: int}.

```
store google pull <dir> [flags]
```

### Examples

```
  store google pull fastlane/metadata
```

### Options

```
  -h, --help   help for pull
```

### Options inherited from parent commands

```
      --app string      package name, e.g. com.example.app (default $GOOGLE_PLAY_PACKAGE_NAME)
  -o, --output string   output format: text, json or toon (default: auto)
  -v, --verbose         log every HTTP request to stderr
```

### SEE ALSO

* [store google](store_google.md)	 - Manage apps in Google Play

