# store - command reference

Every command, flag and example is documented in the generated reference in
[`reference/`](reference/README.md), built from the command help (`store <command> --help`) by
`go generate ./...`. Lookup tables (listing fields, devices, image types, locales, environment
variables) are in [`reference/tables.md`](reference/tables.md), the JSON result objects in
[`reference/results.md`](reference/results.md).

This page keeps what the command help cannot express: conventions, output formats, exit codes,
the metadata directory and a typical CI flow.

Design: `superpowers/specs/2026-10-21-store-cli-design.md`.

## Conventions

### Shape

```
store <apple|google> <noun> <verb> [args] [flags]
```

Verbs mean the same on both sides: `list`, `show`, `create`, `set`, `add`, `remove`, `upload`,
`delete`, `wait`, `lint`, `push`, `pull`.

### Global flags

`--output` and `--verbose` work on every command. `--dry-run` exists on every mutating command.
`store apple` adds `--app` (default `$APPLE_APPSTORE_BUNDLE_ID`) and `--platform`; `store google`
adds `--app` (default `$GOOGLE_PLAY_PACKAGE_NAME`). Each page of the reference lists the flags of
its command, including the inherited ones.

### Text values

Every flag that takes free text accepts the value inline or `@path` to read it from a file
(trailing whitespace trimmed): `--description @metadata/en-US/description.txt`. A literal leading
`@` is written as `@@`.

### Locales

App Store style BCP-47 (`en-US`, `de-DE`, `it`, `es-MX`, `zh-Hans`). The Google side also accepts
Play codes (`it-IT`, `es-419`, `zh-CN`) and maps App Store codes to Play codes where they differ
(see the locale mapping in [tables](reference/tables.md)). A `--release-notes <dir>` directory
holds one `<locale>.txt` per locale and can serve both stores.

### Build references

`<build>` is a build number (`CFBundleVersion` on Apple, `versionCode` on Google) or `latest`.
On Apple a build number can exist under several versions; then `--version <v>` is required.

### Waiting

Commands that wait take `--timeout <duration>` (Go syntax: `90s`, `10m`, `1h`). Progress goes to
stderr. A timeout exits with code 1 and says what it was waiting for.

### Credentials

Credentials come from environment variables, listed in
[tables](reference/tables.md#environment-variables).

## Output

Precedence: `--output`, then `STORE_OUTPUT`, then `toon` if an agent marker is set (see
[tables](reference/tables.md#agent-markers)), else `text`.

- `text`: for humans. Colors when stdout is a terminal and `NO_COLOR` is unset. Lists as tables.
- `json`: the result object, field names as documented below. Stable public interface.
- `toon`: the same object as TOON (Token-Oriented Object Notation), compact for LLMs.

stdout carries only the result. Progress and warnings go to stderr. Errors go to stderr; in `json`
and `toon` mode as an object:

```json
{"error": "version 1.2.0 is in state WAITING_FOR_REVIEW, nothing to submit",
 "status": 409, "code": "STATE_ERROR", "detail": "<API detail>", "hint": "store apple version cancel 1.2.0"}
```

On GitHub Actions (`GITHUB_ACTIONS=true`) errors and warnings are also written to stderr as
[workflow commands](https://docs.github.com/en/actions/reference/workflows-and-actions/workflow-commands),
before the error itself. Each problem of a metadata directory, a release notes directory or an
image file is an annotation of that file (relative to `GITHUB_WORKSPACE`); a text over its limit
is marked at the first character too much:

```
::error file=fastlane/metadata/de-DE/subtitle.txt,line=2,col=20::36 characters, limit 30
```

The last line of the description of each command starts with `Result:` and names the object the command prints
(`[Name]` is a list of it). The fields of every object are in [results.md](reference/results.md),
generated from the result types.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | Success (including "nothing to do") |
| 1 | Failure: API error, validation error, timeout |
| 2 | Usage error: unknown command, bad flag, missing argument |

## Metadata directory

fastlane-compatible, so existing fastlane projects work unchanged:

```
<dir>/
  <locale>/                         App Store, e.g. en-US/, de-DE/: one <file>.txt per listing field
  review_information/               one <file>.txt per App Review field
  android/
    <locale>/                       Google Play, e.g. en-US/, de-DE/, es-419/: one <file>.txt per listing field
      images/<folder or file>       one entry per image type

<screenshots>/                      App Store screenshots, default: <dir>/../screenshots
  <locale>/*.png                    device detected from the pixel size, sorted by file name
  <locale>/<device-alias>/*.png     explicit device, for sizes that fit several devices
```

The file names of every field and the image paths are in [tables](reference/tables.md).
Release notes for both stores can live in a separate directory with one `<locale>.txt` per locale
(`--release-notes <dir>`).

Start a new directory with `store apple init` / `store google init`, or from an existing listing
with `store apple pull` / `store google pull`.
Check it offline, without credentials, with `store apple lint` / `store google lint`: errors are
what the store rejects or needs to submit or publish (exit 1), warnings what Apple and Google
recommend (exit 1 only with `--strict`).

## Typical CI flow

```
# after the iOS build
store apple build upload app.ipa --notes @notes.txt --uses-non-exempt-encryption false
store apple testflight group add-build "Beta" latest

# after the Android build
store google bundle upload app.aab --track internal --release-name "$TAG" --release-notes release-notes

# release
store apple lint fastlane/metadata --release-notes release-notes
store google lint fastlane/metadata --release-notes release-notes
store apple submit "$VERSION" --metadata fastlane/metadata --release-notes release-notes --release after-approval
store google push fastlane/metadata
store google track promote "$TAG" --from internal --to production --rollout 0.2 --release-notes release-notes
```
