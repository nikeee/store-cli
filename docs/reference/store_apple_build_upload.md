## store apple build upload

Upload an IPA to App Store Connect

### Synopsis

Uploads an IPA through the App Store Connect API. Needs no Xcode, altool or Transporter and runs
on Linux. Reads bundle ID, version and build number from the IPA; --app is optional and must
match the IPA when given. Creates a build upload, uploads the file in the parts Apple asks for,
commits it with its MD5 checksum and waits until Apple accepted the upload. FAILED prints
Apple's errors and exits 1; Apple's warnings go to stderr.
--notes sets the TestFlight "What to Test" text and --uses-non-exempt-encryption the export
compliance answer as soon as the build exists. --wait additionally waits until the build is
processed (VALID). --timeout bounds the whole command.
buildId of the result is empty when neither --wait, --notes nor --uses-non-exempt-encryption
made the command wait for the build.
When a step after the accepted upload fails, the error says that the build was uploaded and the
hint names the commands for the remaining steps; json and toon also print the result.
--dry-run reads the IPA, resolves the app and checks that the version has no build with this
number yet (Apple rejects a second upload of it); it uploads nothing.

Result: BuildUpload

```
store apple build upload <ipa> [flags]
```

### Examples

```
  store apple build upload app.ipa --notes @whats-new.txt --uses-non-exempt-encryption false
```

### Options

```
      --dry-run                             print what would change, change nothing
  -h, --help                                help for upload
      --notes string                        TestFlight What to Test text, or @file to read it from a file (max 4000 characters)
      --notes-locale string                 locale of --notes (default "en-US")
      --timeout duration                    how long the whole command may take, e.g. 90s, 10m, 1h (default 30m0s)
      --uses-non-exempt-encryption string   export compliance answer: true or false
      --wait                                also wait until the build is processed (VALID)
```

### Options inherited from parent commands

```
      --app string        bundle ID of the app, e.g. com.example.app (default $APPLE_APPSTORE_BUNDLE_ID)
  -o, --output string     output format: text, json or toon (default: auto)
      --platform string   platform of versions, builds and screenshots: ios, macos, tvos or visionos (default "ios")
  -v, --verbose           log every HTTP request to stderr
```

### SEE ALSO

* [store apple build](store_apple_build.md)	 - List, inspect and update builds

