## store google bundle upload

Upload an AAB and release it on tracks

### Synopsis

Uploads the AAB (resumable upload, streamed from disk), optionally the deobfuscation file for
its version code, and creates one release per --track with that version code, all in one edit.
--status defaults to completed; --rollout implies in-progress; in-progress and halted need --rollout. Apps that were never published
only accept --status draft; the error says so. Release notes max 500 characters per locale;
locales the listing does not have are dropped with a warning.
A completed release replaces the track's releases. An in-progress or halted release keeps the
completed one beside it. A draft keeps the releases that are not drafts.
With --dry-run the edit is created and deleted; nothing is uploaded or changed. The result then
has versionCode 0 and releases without versionCodes.

Result: BundleUploadResult

```
store google bundle upload <aab> [flags]
```

### Examples

```
  store google bundle upload app.aab --track internal --release-name v1.4.0 --release-notes release-notes
```

### Options

```
      --changes-not-sent-for-review   commit without sending the changes for review; needed when Play Console requires sending them manually
      --dry-run                       print what would change, change nothing
  -h, --help                          help for upload
      --mapping string                deobfuscation file (mapping.txt) for the bundle
      --release-name string           release name
      --release-notes string          directory with one <locale>.txt per locale
      --rollout float                 staged rollout fraction, greater than 0 and less than 1; implies --status in-progress
      --status string                 release status: completed, draft, in-progress or halted (default "completed")
      --track stringArray             track to release on: internal, alpha, beta, production or a custom track (repeatable)
```

### Options inherited from parent commands

```
      --app string      package name, e.g. com.example.app (default $GOOGLE_PLAY_PACKAGE_NAME)
  -o, --output string   output format: text, json or toon (default: auto)
  -v, --verbose         log every HTTP request to stderr
```

### SEE ALSO

* [store google bundle](store_google_bundle.md)	 - Upload app bundles

