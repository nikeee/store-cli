# Common workflows

Every mutating line below gets a `--dry-run` run first; the ones marked (ask) reach users or
reviewers and need the user's go after the dry run.

## iOS

```sh
# 1. Upload (any OS). Reads bundle ID, version and build number from the IPA.
store apple build upload app.ipa --notes @notes.txt --uses-non-exempt-encryption false --wait

# 2. TestFlight
store apple testflight group add-build Beta latest          # (ask) testers get it
store apple testflight submit latest                        # (ask) Beta App Review, external groups

# 3. App Store: build, version, metadata, What's New and the review submission in one step
store apple submit 1.4.0 --metadata fastlane/metadata --release-notes release-notes \
  --release after-approval                                  # (ask)

# 4. After approval
store apple version release 1.4.0                           # (ask) only for --release manual
store apple version phased start 1.4.0                      # (ask) 7-day phased release
store apple version phased pause|resume|complete 1.4.0      # complete: (ask) everyone gets it
store apple version cancel 1.4.0                            # withdraw a submission
```

- A build number that exists under several versions needs `--version`.
- `latest` in `submit` is the newest VALID, unexpired build of that version.
- What's New is skipped (with a notice) until the app had a first release: Apple rejects it.
- A version in review is not editable: `submit` exits 1 and the hint names `version cancel`.

## Android

```sh
# 1. Upload and release on tracks, all in one edit
store google bundle upload app.aab --track internal --release-name v1.4.0 --release-notes release-notes

# 2. Promote by release name (default: newest by version code)
store google track promote v1.4.0 --from internal --to production --rollout 0.1   # (ask)

# 3. Staged rollout
store google track rollout production 0.5                   # (ask)
store google track halt production                          # stops the rollout
store google track resume production
store google track complete production                      # (ask) everyone gets it
```

- `completed` replaces the track's releases; `in-progress` and `halted` keep the completed one as
  fallback and need `--rollout`.
- An app that was never published accepts only `--status draft`; the error says so.
- Apps whose changes must be sent for review by hand in the Play Console need
  `--changes-not-sent-for-review`.

## Release notes

One directory serves both stores: `<locale>.txt` per locale, App Store codes (`de-DE`, `es-MX`,
`it`). Play codes are mapped (`es-MX` -> `es-419`); languages the Play listing lacks are dropped
with a warning. Limits: 4000 characters on Apple, 500 on Play, checked before any request.

## Waiting

`build upload --wait` and `build wait <build> --timeout 20m` poll until the build is VALID;
FAILED or INVALID exits 1. Every waiting command takes `--timeout` (`90s`, `10m`, `1h`); a
timeout exits 1 and names what it waited for. On CI runners with a hard limit, set `--timeout`
below it.

## CI (GitHub Actions)

```yaml
- uses: nikeee/store-cli@<tag>        # puts store on the PATH, cached in the runner tool cache
  with:
    version: 0.1.0                    # pin it; default latest
- run: store apple build upload app.ipa --wait
  env:
    APPLE_APPSTORE_ISSUER_ID: ${{ secrets.APPLE_APPSTORE_ISSUER_ID }}
    APPLE_APPSTORE_API_KEY_ID: ${{ secrets.APPLE_APPSTORE_API_KEY_ID }}
    APPLE_APPSTORE_API_PRIVATE_KEY: ${{ secrets.APPLE_APPSTORE_API_PRIVATE_KEY }}
    APPLE_APPSTORE_BUNDLE_ID: com.example.app
```

The action runs on Linux and macOS runners (X64, ARM64). Store the credentials as repository
secrets and pass them as environment variables; nothing else is needed.
