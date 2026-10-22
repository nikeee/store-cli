## store google images delete

Delete all images of one locale and type

### Synopsis

Deletes all images of the type in the locale. A type without images is a skip (exit 0). --locale
takes an App Store style locale. Types: phone, tablet-7, tablet-10, tv, wear, feature-graphic, icon, promo-graphic, tv-banner.
With --dry-run the result is the planned Plan.

Result: ImagesDeleted

```
store google images delete --locale <l> --type <t> [flags]
```

### Examples

```
  store google images delete --locale en-US --type tv-banner
```

### Options

```
      --changes-not-sent-for-review   commit without sending the changes for review; needed when Play Console requires sending them manually
      --dry-run                       print what would change, change nothing
  -h, --help                          help for delete
      --locale string                 locale of the images, e.g. en-US (required)
      --type string                   image type (required)
```

### Options inherited from parent commands

```
      --app string      package name, e.g. com.example.app (default $GOOGLE_PLAY_PACKAGE_NAME)
  -o, --output string   output format: text, json or toon (default: auto)
  -v, --verbose         log every HTTP request to stderr
```

### SEE ALSO

* [store google images](store_google_images.md)	 - List, upload and delete listing images

