## store apple submit

Do the whole App Store submission of a version

### Synopsis

The whole App Store submission in one command:
1. Find the build (--build, default latest = newest VALID, unexpired build of <version>).
2. Find or create the version. It must be editable (PREPARE_FOR_SUBMISSION, DEVELOPER_REJECTED,
   REJECTED, METADATA_REJECTED, INVALID_BINARY); otherwise exit 1 with its state.
3. Attach the build. Set the release type only when --release is given (scheduled needs
   --release-date, an RFC 3339 time; --release-date needs --release scheduled).
4. --metadata: run push against this version.
5. --release-notes: set "What's New" per locale. While the app has never had a released version,
   this step is skipped with a notice (Apple rejects the field then).
6. Submit as in version submit.
Local inputs are checked before the first API call. --dry-run changes nothing and shows the steps.

Result: SubmitResult

```
store apple submit <version> [--build N|latest] [--metadata <dir>] [--screenshots <dir>] [--release-notes <dir>] [--release manual|after-approval|scheduled] [--release-date <time>] [flags]
```

### Examples

```
  store apple submit 1.4.0 --metadata fastlane/metadata --release-notes release-notes --release after-approval
```

### Options

```
      --build string           build number to submit, or latest (newest valid, unexpired build of the version) (default "latest")
      --dry-run                print what would change, change nothing
  -h, --help                   help for submit
      --metadata string        metadata directory to push to the version first
      --release string         how the version is released: manual, after-approval or scheduled
      --release-date string    release time for --release scheduled, an RFC 3339 time
      --release-notes string   directory with <locale>.txt What's New texts
      --screenshots string     screenshots directory (default: <dir>/../screenshots)
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

