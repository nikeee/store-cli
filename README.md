# store

A command line tool for App Store Connect and Google Play. One tool, the same verbs for both stores:
upload builds, manage TestFlight, submit versions, push listings and screenshots, and roll out
releases. It talks to the store APIs directly, so it needs no Xcode, `altool`, Transporter or
Ruby and runs on Linux CI.

## Install

Download the archive for your platform from the
[GitHub Releases](https://github.com/nikeee/store-cli/releases) (Linux and macOS, amd64 and arm64),
unpack it and put `store` on your `PATH`. Or build it with Go:

```
go install github.com/nikeee/store-cli/cmd/store@latest
```

Check it with `store version`.

## Credentials

Credentials come from environment variables.

### App Store Connect

1. In App Store Connect open Users and Access, Integrations, App Store Connect API and create a
   team API key with the App Manager role.
2. Note the Issuer ID and the Key ID, and download the `.p8` key (it can be downloaded once).
3. Export them:

```
export APPLE_APPSTORE_ISSUER_ID=<issuer id>
export APPLE_APPSTORE_API_KEY_ID=<key id>
export APPLE_APPSTORE_API_PRIVATE_KEY=/path/to/AuthKey_<key id>.p8   # or the PEM content
export APPLE_APPSTORE_BUNDLE_ID=com.example.app                      # default for --app
```

### Google Play

1. In Google Cloud create a service account and a JSON key for it, and enable the Google Play
   Android Developer API for the project.
2. In the Play Console open Users and permissions and invite the service account's email address
   with the permissions for releases and for the store listing of your app.
3. Export the key and the package name:

```
export GOOGLE_PLAY_SERVICE_ACCOUNT_JSON=/path/to/key.json   # or the JSON content
export GOOGLE_PLAY_PACKAGE_NAME=com.example.app             # default for --app
```

All variables are listed in [docs/reference/tables.md](docs/reference/tables.md#environment-variables).

## Quickstart

```
# check the credentials
store apple auth check
store google auth check

# see what is there
store apple app show
store google track list

# upload a build and wait until Apple processed it
store apple build upload app.ipa --wait --uses-non-exempt-encryption false
store google bundle upload app.aab --track internal --release-notes release-notes

# keep the store listings in a fastlane-style directory
store apple pull fastlane/metadata
store apple lint fastlane/metadata
store apple push fastlane/metadata --dry-run
store google push fastlane/metadata --dry-run
```

Every mutating command accepts `--dry-run`, which prints what would change and changes nothing.
Start a metadata directory from scratch with `store apple init` and `store google init`.

## Examples

Get a build to your TestFlight testers:

```
store apple build upload app.ipa --wait --notes @notes.txt --uses-non-exempt-encryption false
store apple testflight group add-build Beta latest
```

Submit 1.4.0 for review with listing and What's New, release when Apple approves it:

```
store apple submit 1.4.0 --metadata fastlane/metadata --release-notes release-notes --release after-approval
```

Staged rollout on Play, and the brake if something goes wrong:

```
store google track promote v1.4.0 --from internal --to production --rollout 0.1
store google track rollout production 0.5
store google track halt production
store google track complete production
```

## GitHub Actions

The setup action of this repository downloads a release, checks its checksum, puts `store` on the
`PATH` and keeps it in the runner's tool cache (self-hosted runners reuse it between jobs). It runs
on Linux and macOS runners, X64 and ARM64.

```yaml
- uses: nikeee/store-cli@main
  with:
    version: 0.1.1 # or latest (the default); pin it for reproducible builds
```

Outputs: `version` (the installed version) and `cache-hit`. The `token` input (default: the
workflow's token) is used to download the release.

On GitHub Actions (`GITHUB_ACTIONS=true`) errors and warnings also go to stderr as workflow
commands, so they show as annotations. Problems in the metadata directory, the release notes and
image files point at the file, and for texts over a limit at the line and column of the first
character too much.

A release workflow that runs on every `v*` tag, uploads both builds and submits them. The builds
themselves are up to you.

```yaml
name: release
on:
  push:
    tags: ['v*']

jobs:
  release:
    runs-on: ubuntu-latest
    env:
      APPLE_APPSTORE_ISSUER_ID: ${{ secrets.APPLE_APPSTORE_ISSUER_ID }}
      APPLE_APPSTORE_API_KEY_ID: ${{ secrets.APPLE_APPSTORE_API_KEY_ID }}
      APPLE_APPSTORE_API_PRIVATE_KEY: ${{ secrets.APPLE_APPSTORE_API_PRIVATE_KEY }}
      APPLE_APPSTORE_BUNDLE_ID: com.example.app
      GOOGLE_PLAY_SERVICE_ACCOUNT_JSON: ${{ secrets.GOOGLE_PLAY_SERVICE_ACCOUNT_JSON }}
      GOOGLE_PLAY_PACKAGE_NAME: com.example.app
    steps:
      - uses: actions/checkout@v4
      - uses: nikeee/store-cli@main
        with:
          version: 0.1.1

      # after the iOS and Android builds
      - run: store apple build upload app.ipa --notes @notes.txt --uses-non-exempt-encryption false
      - run: store apple testflight group add-build "Beta" latest
      - run: store google bundle upload app.aab --track internal --release-name "$GITHUB_REF_NAME" --release-notes release-notes

      # release
      - run: store apple lint fastlane/metadata --release-notes release-notes
      - run: store google lint fastlane/metadata --release-notes release-notes
      - run: store apple submit "${GITHUB_REF_NAME#v}" --metadata fastlane/metadata --release-notes release-notes --release after-approval
      - run: store google push fastlane/metadata
      - run: store google track promote "$GITHUB_REF_NAME" --from internal --to production --rollout 0.2 --release-notes release-notes
```

## Other CI

`store` is a single static binary. Any runner with `curl` can fetch it (`checksums.txt` sits next
to the archives):

```
curl -sSfL https://github.com/nikeee/store-cli/releases/download/v0.1.1/store_0.1.1_linux_amd64.tar.gz \
  | tar -xz -C /usr/local/bin store
```

## Expo

No EAS needed. `expo prebuild` writes the native projects, Gradle and Xcode build them on the
runner and `store` uploads the results. iOS needs macOS, so the job runs both builds there. Map
the store credentials into `env` as in the example above, plus `UPLOAD_KEYSTORE` (the Play upload
keystore, base64), `UPLOAD_KEYSTORE_PASSWORD` and `UPLOAD_KEY_ALIAS`. `MyApp` is the `name` from
`app.json`.

```yaml
name: release
on:
  push:
    tags: ['v*']

jobs:
  release:
    runs-on: macos-15
    steps:
      - uses: actions/checkout@v4
      - uses: actions/setup-node@v4
        with:
          node-version: 22
          cache: npm
      - uses: actions/setup-java@v4
        with:
          distribution: temurin
          java-version: 17
      - run: npm ci
      - run: npx expo prebuild
      - uses: nikeee/store-cli@main
        with:
          version: 0.1.1

      - name: Android
        run: |
          echo "$UPLOAD_KEYSTORE" | base64 -d > "$RUNNER_TEMP/upload.jks"
          (cd android && ./gradlew bundleRelease \
            -Pandroid.injected.signing.store.file="$RUNNER_TEMP/upload.jks" \
            -Pandroid.injected.signing.store.password="$UPLOAD_KEYSTORE_PASSWORD" \
            -Pandroid.injected.signing.key.alias="$UPLOAD_KEY_ALIAS" \
            -Pandroid.injected.signing.key.password="$UPLOAD_KEYSTORE_PASSWORD")
          store google bundle upload android/app/build/outputs/bundle/release/app-release.aab \
            --track internal --release-name "$GITHUB_REF_NAME" --release-notes release-notes

      - name: iOS
        run: |
          echo "$APPLE_APPSTORE_API_PRIVATE_KEY" > "$RUNNER_TEMP/key.p8"
          sign="-allowProvisioningUpdates -authenticationKeyPath $RUNNER_TEMP/key.p8 -authenticationKeyID $APPLE_APPSTORE_API_KEY_ID -authenticationKeyIssuerID $APPLE_APPSTORE_ISSUER_ID"
          xcodebuild archive -workspace ios/MyApp.xcworkspace -scheme MyApp -configuration Release \
            -destination generic/platform=iOS -archivePath build/MyApp.xcarchive $sign
          /usr/libexec/PlistBuddy -c "Add :method string app-store-connect" build/export.plist
          xcodebuild -exportArchive -archivePath build/MyApp.xcarchive -exportPath build \
            -exportOptionsPlist build/export.plist $sign
          store apple build upload build/MyApp.ipa --wait
          store apple testflight group add-build Beta latest
```

Set `ios.appleTeamId` and `ios.config.usesNonExemptEncryption` in `app.json`. The first lets Xcode
sign automatically, the second answers export compliance, so the upload needs no
`--uses-non-exempt-encryption`. Xcode signs with the same App Store Connect API key, which needs
the Admin role for that.

## Output and LLM agents

Results go to stdout, progress and errors to stderr. `--output` (or `STORE_OUTPUT`) selects the
format:

- `text`: for humans, with colors on a terminal and tables for lists.
- `json`: the result object, a stable interface for scripts.
- `toon`: the same object as [TOON](https://github.com/toon-format/toon), compact for LLMs.

When an AI coding agent is detected from its environment (for example `CLAUDECODE` or
`AI_AGENT`), the default is `toon`. Errors are objects with `error`, `status`, `code` and `hint` in
`json` and `toon` mode. Exit codes: 0 success, 1 failure, 2 usage error.

## Agent skill

`skills/store-cli/` is an [Agent Skill](https://agentskills.io) that teaches coding agents to use
`store` safely: dry run first, confirm before anything reaches users or reviewers, never touch the
credentials. Install it with `npx skills add nikeee/store-cli`, or copy the folder into
`.claude/skills/` (Claude Code) or `.agents/skills/`.

## Documentation

- [Command reference](docs/reference/README.md), generated from the command help, with
  [lookup tables](docs/reference/tables.md) for fields, devices, image types and locales.
- [Conventions, output, exit codes and the metadata directory](docs/commands.md).

## License

MIT, see [LICENSE](LICENSE).
