## store apple screenshots upload

Upload screenshots to the set of one locale and device

### Synopsis

Uploads the files in the given order. The device comes from --device or from the pixel size of the
first file (all files must be the same device); an unknown size lists the accepted sizes. Where a
size fits several devices the first one in the device table wins; pass --device to choose. Files are
PNG or JPEG without an alpha channel, checked locally before any API call.
Without --replace the files are appended to the set (max 10 per set). With --replace: if the set
already holds exactly these files in this order (MD5), nothing happens; otherwise the set is emptied
and the files are uploaded in order. Each file is reserved, uploaded, committed with its MD5 and
polled until Apple processed it; --timeout is the time per file. With
--dry-run the result is the planned Plan.

Result: ScreenshotSet

```
store apple screenshots upload --locale <l> [--device <d>] [--version <v>] [--replace] <png>... [flags]
```

### Examples

```
  store apple screenshots upload --locale en-US --replace shots/en-US/*.png
  store apple screenshots upload --locale de-DE --device ipad-12.9 --timeout 20m ipad1.png ipad2.png
```

### Options

```
      --device string      device of the set, e.g. iphone-6.9 (default: detected from the first file)
      --dry-run            print what would change, change nothing
  -h, --help               help for upload
      --locale string      locale of the set, e.g. en-US (required)
      --replace            make the set hold exactly these files instead of appending
      --timeout duration   how long one file may take until Apple processed it, e.g. 90s, 10m, 1h (default 10m0s)
      --version string     app version (default: the editable version, else the live one)
```

### Options inherited from parent commands

```
      --app string        bundle ID of the app, e.g. com.example.app (default $APPLE_APPSTORE_BUNDLE_ID)
  -o, --output string     output format: text, json or toon (default: auto)
      --platform string   platform of versions, builds and screenshots: ios, macos, tvos or visionos (default "ios")
  -v, --verbose           log every HTTP request to stderr
```

### SEE ALSO

* [store apple screenshots](store_apple_screenshots.md)	 - List, upload and delete screenshots

