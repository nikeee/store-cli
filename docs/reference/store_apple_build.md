## store apple build

List, inspect and update builds

```
store apple build [flags]
```

### Options

```
  -h, --help   help for build
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
* [store apple build expire](store_apple_build_expire.md)	 - Expire a build in TestFlight
* [store apple build list](store_apple_build_list.md)	 - List builds, newest first
* [store apple build set-encryption](store_apple_build_set-encryption.md)	 - Set the export compliance answer of a build
* [store apple build show](store_apple_build_show.md)	 - Show one build
* [store apple build upload](store_apple_build_upload.md)	 - Upload an IPA to App Store Connect
* [store apple build wait](store_apple_build_wait.md)	 - Wait until a build exists and is processed

