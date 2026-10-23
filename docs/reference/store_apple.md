## store apple

Manage apps in App Store Connect

### Synopsis

Manage apps in App Store Connect with a team API key. Credentials come from the environment:
APPLE_APPSTORE_ISSUER_ID, APPLE_APPSTORE_API_KEY_ID and APPLE_APPSTORE_API_PRIVATE_KEY.

```
store apple [flags]
```

### Options

```
      --app string        bundle ID of the app, e.g. com.example.app (default $APPLE_APPSTORE_BUNDLE_ID)
  -h, --help              help for apple
      --platform string   platform of versions, builds and screenshots: ios, macos, tvos or visionos (default "ios")
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json or toon (default: auto)
  -v, --verbose         log every HTTP request to stderr
```

### SEE ALSO

* [store](store.md)	 - Manage App Store Connect and Google Play from the command line
* [store apple app](store_apple_app.md)	 - Show the apps of the team
* [store apple auth](store_apple_auth.md)	 - Check the App Store Connect credentials
* [store apple build](store_apple_build.md)	 - List, inspect and update builds
* [store apple init](store_apple_init.md)	 - Scaffold the metadata directory for a new app
* [store apple lint](store_apple_lint.md)	 - Check a metadata directory offline
* [store apple listing](store_apple_listing.md)	 - Show and change the store listing texts
* [store apple previews](store_apple_previews.md)	 - List, upload and delete app previews
* [store apple pull](store_apple_pull.md)	 - Write the store listing into a metadata directory
* [store apple push](store_apple_push.md)	 - Make App Store Connect match a metadata directory
* [store apple release-notes](store_apple_release-notes.md)	 - Set the What's New text
* [store apple review-info](store_apple_review-info.md)	 - Show or change the App Review information
* [store apple screenshots](store_apple_screenshots.md)	 - List, upload and delete screenshots
* [store apple submit](store_apple_submit.md)	 - Do the whole App Store submission of a version
* [store apple testflight](store_apple_testflight.md)	 - Manage TestFlight
* [store apple version](store_apple_version.md)	 - Manage App Store versions

