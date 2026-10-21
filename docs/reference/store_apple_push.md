## store apple push

Make App Store Connect match a metadata directory

### Synopsis

Makes the store match the directory (fastlane layout):
- Target version: --version, else the one editable version of --platform. None: exit 1.
- A file that exists is the desired value; an empty file clears the field (name.txt must not be
  empty, an empty demo_required.txt changes nothing); a missing file leaves the field alone.
- A locale folder without a store localization creates it. Store locales without a folder are
  left alone.
- Screenshots (default <dir>/../screenshots): per locale and device, the ordered MD5 list is
  compared; any difference replaces the whole set. Devices without local files are left alone.
- review_information/ sets the App Review details.
- What's New (release_notes.txt) is skipped with a warning while the app was never released.
All texts and images are checked before any API call. First the plan is computed and printed
(stderr in text mode), then applied; --dry-run stops after the plan. Unchanged values cause no
API call. When a change fails, push stops with exit 1 and the plan shows which changes were
applied (skip for the others); a screenshot set changed only partly is reported incomplete, run
push again. The action of each change shows what was done.

Result: Plan

```
store apple push <dir> [--screenshots <dir>] [--version <v>] [--only texts,screenshots,review] [flags]
```

### Examples

```
  store apple push fastlane/metadata --dry-run
  store apple push fastlane/metadata --only texts,review
```

### Options

```
      --dry-run              print what would change, change nothing
  -h, --help                 help for push
      --only strings         parts to push, comma separated: texts, screenshots, review (default: all)
      --screenshots string   screenshots directory (default: <dir>/../screenshots)
      --version string       app version to change (default: the one editable version)
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

