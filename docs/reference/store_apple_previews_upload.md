## store apple previews upload

Upload app previews to the set of one locale and device

### Synopsis

Uploads the videos (.mp4, .mov or .m4v) in the given order. Unlike screenshots the device is required,
as there is no size detection for videos; watches have no previews. The preview type is the display
type without APP_ (IPHONE_67 for APP_IPHONE_67). Without --replace the files are
appended to the set (max 3 per set). With --replace: if the set already holds exactly these files in
this order (MD5), nothing happens; otherwise the set is emptied and the files are uploaded in order.
--poster-frame is the timecode of the poster frame (e.g. 00:00:05:00), sent for every file. Each file
is reserved, uploaded, committed with its MD5 and polled until Apple processed it; --timeout is the
time per file. With --dry-run the result is the planned Plan.

Result: PreviewSet

```
store apple previews upload --locale <l> --device <d> [--version <v>] [--replace] [--poster-frame <timecode>] <video>... [flags]
```

### Examples

```
  store apple previews upload --locale en-US --device iphone-6.9 --replace previews/en-US.mp4
  store apple previews upload --locale de-DE --device ipad-13 --poster-frame 00:00:05:00 --timeout 30m tour.mov
```

### Options

```
      --device string         device of the set, e.g. iphone-6.9 (required)
      --dry-run               print what would change, change nothing
  -h, --help                  help for upload
      --locale string         locale of the set, e.g. en-US (required)
      --poster-frame string   timecode of the poster frame, e.g. 00:00:05:00
      --replace               make the set hold exactly these files instead of appending
      --timeout duration      how long one file may take until Apple processed it, e.g. 90s, 10m, 1h (default 10m0s)
      --version string        app version (default: the editable version, else the live one)
```

### Options inherited from parent commands

```
      --app string        bundle ID of the app, e.g. com.example.app (default $APPLE_APPSTORE_BUNDLE_ID)
  -o, --output string     output format: text, json or toon (default: auto)
      --platform string   platform of versions, builds and screenshots: ios, macos, tvos or visionos (default "ios")
  -v, --verbose           log every HTTP request to stderr
```

### SEE ALSO

* [store apple previews](store_apple_previews.md)	 - List, upload and delete app previews

