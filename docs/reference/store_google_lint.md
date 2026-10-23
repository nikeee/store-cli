## store google lint

Check a metadata directory offline

### Synopsis

Checks <dir>/android/ without credentials or network and reports every problem at once.
Errors:
- everything push rejects: text limits, folders not named by Play codes, image format, alpha
  channel, size, aspect ratio and count, an icon over 1024 KB
- with --release-notes: the release notes in that directory
Warnings:
- no feature graphic, or no locale with 2 screenshots (required to publish)
- phone sets under 4 screenshots, phone screenshots not 9:16 or 16:9 with 1080 px (needed
  for promotion)
- titles with emoji or emoticons, all capitals, repeated special characters, or ranking and
  promotion terms (#1, best, free, % off)
A missing file or image is no finding: push leaves it alone. Exit 1 with errors, with --strict
also with warnings. On GitHub Actions every finding is also an annotation at its file.

Result: LintResult

```
store google lint <dir> [--only texts,images] [--release-notes <dir>] [--strict] [flags]
```

### Examples

```
  store google lint fastlane/metadata
  store google lint fastlane/metadata --release-notes release-notes --strict
```

### Options

```
  -h, --help                   help for lint
      --only strings           parts to check, comma separated: texts, images (default: all)
      --release-notes string   also check this directory with one <locale>.txt per locale
      --strict                 exit 1 on warnings too
```

### Options inherited from parent commands

```
      --app string      package name, e.g. com.example.app (default $GOOGLE_PLAY_PACKAGE_NAME)
  -o, --output string   output format: text, json or toon (default: auto)
  -v, --verbose         log every HTTP request to stderr
```

### SEE ALSO

* [store google](store_google.md)	 - Manage apps in Google Play

