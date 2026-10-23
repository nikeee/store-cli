## store google

Manage apps in Google Play

### Synopsis

Manage apps in Google Play with a service account. The credentials come from the environment:
GOOGLE_PLAY_SERVICE_ACCOUNT_JSON (the service account needs access to the app in the Play Console).
Every mutating command runs in its own edit: create, change, commit. If anything fails, the edit is
deleted and nothing is published. Only a commit that gets no answer from Play may have been applied:
then the error says so, check Play Console. Mutating commands accept --changes-not-sent-for-review (passed to
the commit; needed when the Play Console requires changes to be sent for review manually).

```
store google [flags]
```

### Options

```
      --app string   package name, e.g. com.example.app (default $GOOGLE_PLAY_PACKAGE_NAME)
  -h, --help         help for google
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json or toon (default: auto)
  -v, --verbose         log every HTTP request to stderr
```

### SEE ALSO

* [store](store.md)	 - Manage App Store Connect and Google Play from the command line
* [store google auth](store_google_auth.md)	 - Check the Google Play credentials
* [store google bundle](store_google_bundle.md)	 - Upload app bundles
* [store google details](store_google_details.md)	 - Show or change the app contact details
* [store google images](store_google_images.md)	 - List, upload and delete listing images
* [store google init](store_google_init.md)	 - Scaffold the Play listing directory for a new app
* [store google lint](store_google_lint.md)	 - Check a metadata directory offline
* [store google listing](store_google_listing.md)	 - Show and change the store listing texts
* [store google pull](store_google_pull.md)	 - Write the Play listing into a metadata directory
* [store google push](store_google_push.md)	 - Make the Play listing match a metadata directory
* [store google release-notes](store_google_release-notes.md)	 - Set the release notes of a release
* [store google track](store_google_track.md)	 - Inspect and promote release tracks

