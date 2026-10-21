## store google images upload

Upload images of one locale and type

### Synopsis

Uploads PNG or JPEG files in the given order. Format, alpha channel, pixel size and aspect ratio
and the number of files are checked locally before any API call. Without --replace the files are
appended (at most the maximum of the type per locale). With --replace: if Play already holds exactly
these files in this order (SHA-256), nothing happens; otherwise all images of the type are deleted
and the files uploaded in order. Single-image types (feature-graphic, icon, promo-graphic,
tv-banner) take exactly one file and always replace their image; phone needs 2 to 8 screenshots.
If an upload fails after images were deleted, the whole edit is discarded, so nothing is published;
run the command again. --locale takes an App Store style locale. Types: phone, tablet-7, tablet-10, tv, wear, feature-graphic, icon, promo-graphic, tv-banner.
With --dry-run the result is the planned Plan.

Result: ImageSet

```
store google images upload --locale <l> --type <t> [--replace] <image>... [flags]
```

### Examples

```
  store google images upload --locale en-US --type phone --replace shots/1.png shots/2.png
  store google images upload --locale en-US --type feature-graphic feature.png
```

### Options

```
      --changes-not-sent-for-review   commit without sending the changes for review; needed when Play Console requires sending them manually
      --dry-run                       print what would change, change nothing
  -h, --help                          help for upload
      --locale string                 locale of the images, e.g. en-US (required)
      --replace                       make the images of the type exactly these files instead of appending
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

