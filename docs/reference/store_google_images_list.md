## store google images list

List the images of a locale

### Synopsis

Lists the images of one locale in Play's order with their ID, SHA-256 and preview URL. Without
--type every image type that holds images is listed. --locale takes an App Store style locale,
sent as the Play language code. Types: phone, tablet-7, tablet-10, tv, wear, feature-graphic, icon, promo-graphic, tv-banner.

Result: [ImageSet]

```
store google images list --locale <l> [--type <t>] [flags]
```

### Examples

```
  store google images list --locale en-US --type phone
```

### Options

```
  -h, --help            help for list
      --locale string   locale to list, e.g. en-US (required)
      --type string     only this image type
```

### Options inherited from parent commands

```
      --app string      package name, e.g. com.example.app (default $GOOGLE_PLAY_PACKAGE_NAME)
  -o, --output string   output format: text, json or toon (default: auto)
  -v, --verbose         log every HTTP request to stderr
```

### SEE ALSO

* [store google images](store_google_images.md)	 - List, upload and delete listing images

