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
store apple push fastlane/metadata --dry-run
store google push fastlane/metadata --dry-run
```

Every mutating command accepts `--dry-run`, which prints what would change and changes nothing.
Start a metadata directory from scratch with `store apple init` and `store google init`.

## GitHub Actions

A release job that uploads both builds and submits them. The builds themselves are up to you.

```yaml
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
      - uses: actions/setup-go@v5
        with:
          go-version: stable
      - run: go install github.com/nikeee/store-cli/cmd/store@latest

      # after the iOS and Android builds
      - run: store apple build upload app.ipa --notes @notes.txt --uses-non-exempt-encryption false
      - run: store apple testflight group add-build "Beta" latest
      - run: store google bundle upload app.aab --track internal --release-name "$TAG" --release-notes release-notes

      # release
      - run: store apple submit "$VERSION" --metadata fastlane/metadata --release-notes release-notes --release after-approval
      - run: store google push fastlane/metadata
      - run: store google track promote "$TAG" --from internal --to production --rollout 0.2 --release-notes release-notes
```

## Output and LLM agents

Results go to stdout, progress and errors to stderr. `--output` (or `STORE_OUTPUT`) selects the
format:

- `text`: for humans, with colors on a terminal and tables for lists.
- `json`: the result object, a stable interface for scripts.
- `toon`: the same object as [TOON](https://github.com/toon-format/toon), compact for LLMs.

When an AI coding agent is detected from its environment (for example `CLAUDECODE` or
`AI_AGENT`), the default is `toon`. Errors are objects with `error`, `status`, `code` and `hint` in
`json` and `toon` mode. Exit codes: 0 success, 1 failure, 2 usage error.

## Documentation

- [Command reference](docs/reference/README.md), generated from the command help, with
  [lookup tables](docs/reference/tables.md) for fields, devices, image types and locales.
- [Conventions, output, exit codes and the metadata directory](docs/commands.md).

## License

MIT, see [LICENSE](LICENSE).
