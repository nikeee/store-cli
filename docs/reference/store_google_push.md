## store google push

Make the Play listing match a metadata directory

### Synopsis

Makes the Play listing match <dir>/android/ (fastlane layout, locale folders named by Play codes):
- A file that exists is the desired value; an empty file clears the field; a missing file leaves
  the field alone.
- A locale folder without a Play listing creates it (its empty files are skipped). Play languages
  without a folder are left alone; no language is deleted.
- Images (<locale>/images/): per type, the ordered SHA-256 list is compared with Play's; any
  difference replaces all images of that type. Types without local files are left alone.
- Release notes are not part of push.
All texts and images are checked before any API call. Everything runs in one edit: first the plan
is computed and printed (stderr in text mode), then the listings and the images are changed and the
edit is committed; --dry-run stops after the plan. With nothing to change nothing is committed.
When anything fails, the edit is deleted, so nothing is published: push exits 1 and the plan shows
which changes had been staged (skip for the others). Result: Plan.

```
store google push <dir> [--only texts,images] [flags]
```

### Examples

```
  store google push fastlane/metadata --dry-run
  store google push fastlane/metadata --only texts
```

### Options

```
      --changes-not-sent-for-review   commit without sending the changes for review; needed when Play Console requires sending them manually
      --dry-run                       print what would change, change nothing
  -h, --help                          help for push
      --only strings                  parts to push, comma separated: texts, images (default: all)
```

### Options inherited from parent commands

```
      --app string      package name, e.g. com.example.app (default $GOOGLE_PLAY_PACKAGE_NAME)
  -o, --output string   output format: text, json or toon (default: auto)
  -v, --verbose         log every HTTP request to stderr
```

### SEE ALSO

* [store google](store_google.md)	 - Manage apps in Google Play

