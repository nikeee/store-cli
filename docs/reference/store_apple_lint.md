## store apple lint

Check a metadata directory offline

### Synopsis

Checks the metadata directory and its screenshots (default <dir>/../screenshots) without
credentials or network and reports every problem at once.
Errors:
- everything push rejects: text limits (keywords in bytes), a name under 2 characters,
  demo_required.txt not true or false, screenshot format, alpha channel, size and count,
  unknown device folders
- an empty description.txt, keywords.txt or support_url.txt (required to submit)
- Apple Watch screenshots of more than one size across the locales
- with --release-notes: the release notes in that directory
Warnings:
- no iphone-6.9 or iphone-6.5 screenshots (--platform ios); iPad screenshots without ipad-13
- keywords shorter than 3 characters, spaces next to commas, repeated keywords, the word
  "app", words of the name or subtitle
A missing file or image is no finding: push leaves it alone. Exit 1 with errors, with --strict
also with warnings. On GitHub Actions every finding is also an annotation at its file.

Result: LintResult

```
store apple lint <dir> [--screenshots <dir>] [--only texts,screenshots,review] [--release-notes <dir>] [--strict] [flags]
```

### Examples

```
  store apple lint fastlane/metadata
  store apple lint fastlane/metadata --release-notes release-notes --strict
```

### Options

```
  -h, --help                   help for lint
      --only strings           parts to check, comma separated: texts, screenshots, review (default: all)
      --release-notes string   also check this directory with one <locale>.txt per locale
      --screenshots string     screenshots directory (default: <dir>/../screenshots)
      --strict                 exit 1 on warnings too
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

