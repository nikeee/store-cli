# store - command reference

This is the specification of every command, flag and result. It is the source for the cobra help
texts. Once the commands exist, the reference in `docs/reference/` is generated from cobra
(`go generate ./...`) and this file is reduced to the parts cobra cannot express (layouts, tables,
conventions), so nothing is maintained twice.

Design: `docs/superpowers/specs/2026-10-21-store-cli-design.md`.

## Conventions

### Shape

```
store <apple|google> <noun> <verb> [args] [flags]
```

Verbs mean the same on both sides: `list`, `show`, `create`, `set`, `add`, `remove`, `upload`,
`delete`, `wait`, `push`, `pull`.

### Global flags

| Flag | Type | Default | Meaning |
|---|---|---|---|
| `-o, --output` | `text\|json\|toon` | auto | Output format, see "Output" |
| `-v, --verbose` | bool | false | Log every HTTP request (method, path, status, duration) to stderr |
| `--dry-run` | bool | false | Only on mutating commands: print what would change, change nothing |

Under `store apple`:

| Flag | Type | Default | Meaning |
|---|---|---|---|
| `--app` | string | `$APPLE_APPSTORE_BUNDLE_ID` | Bundle ID of the app, e.g. `com.example.app` |
| `--platform` | `ios\|macos\|tvos\|visionos` | `ios` | Platform of versions, builds and screenshots |

Under `store google`:

| Flag | Type | Default | Meaning |
|---|---|---|---|
| `--app` | string | `$GOOGLE_PLAY_PACKAGE_NAME` | Package name, e.g. `com.example.app` |

### Text values

Every flag that takes free text accepts the value inline or `@path` to read it from a file
(trailing whitespace trimmed): `--description @metadata/en-US/description.txt`. A literal leading
`@` is written as `@@`.

### Locales

App Store style BCP-47 (`en-US`, `de-DE`, `it`, `es-MX`, `zh-Hans`). The Google side also accepts
Play codes (`it-IT`, `es-419`, `zh-CN`) and maps App Store codes to Play codes where they differ.
A `--release-notes <dir>` directory holds one `<locale>.txt` per locale and can serve both stores.

### Build references

`<build>` is a build number (`CFBundleVersion` on Apple, `versionCode` on Google) or `latest`.
On Apple a build number can exist under several versions; then `--version <v>` is required.

### Waiting

Commands that wait take `--timeout <duration>` (Go syntax: `90s`, `10m`, `1h`). Progress goes to
stderr. A timeout exits with code 1 and says what it was waiting for.

### Environment variables

| Variable | Used by | Content |
|---|---|---|
| `APPLE_APPSTORE_ISSUER_ID` | `store apple` | Issuer ID from App Store Connect, Users and Access, Integrations |
| `APPLE_APPSTORE_API_KEY_ID` | `store apple` | Key ID of the team API key |
| `APPLE_APPSTORE_API_PRIVATE_KEY` | `store apple` | The `.p8` key: PEM content, or a path to the file |
| `APPLE_APPSTORE_BUNDLE_ID` | `store apple` | Default for `--app` |
| `GOOGLE_PLAY_SERVICE_ACCOUNT_JSON` | `store google` | Service account key: JSON content, or a path to the file |
| `GOOGLE_PLAY_PACKAGE_NAME` | `store google` | Default for `--app` |
| `STORE_OUTPUT` | all | Default for `--output` |
| `NO_COLOR` | all | Disables colors in text output |
| `AI_AGENT`, `CLAUDECODE`, ... | all | Agent markers; any of them set switches the default output to `toon` |

### Output

Precedence: `--output`, then `STORE_OUTPUT`, then `toon` if an agent marker is set, else `text`.

- `text`: for humans. Colors when stdout is a terminal and `NO_COLOR` is unset. Lists as tables.
- `json`: the result object, field names as documented below. Stable public interface.
- `toon`: the same object as TOON (Token-Oriented Object Notation), compact for LLMs.

stdout carries only the result. Progress and warnings go to stderr. Errors go to stderr; in `json`
and `toon` mode as an object:

```json
{"error": "version 1.2.0 is in state WAITING_FOR_REVIEW, nothing to submit",
 "status": 409, "code": "STATE_ERROR", "detail": "<API detail>", "hint": "store apple version cancel 1.2.0"}
```

### Exit codes

| Code | Meaning |
|---|---|
| 0 | Success (including "nothing to do") |
| 1 | Failure: API error, validation error, timeout |
| 2 | Usage error: unknown command, bad flag, missing argument |

---

## store

```
store version
store completion <bash|zsh|fish>
```

`version` prints `{version, commit, date}` (from the release build; `dev` for local builds).

---

## store apple

Credentials: `APPLE_APPSTORE_ISSUER_ID`, `APPLE_APPSTORE_API_KEY_ID`,
`APPLE_APPSTORE_API_PRIVATE_KEY`.

### Result objects

```
App          {id, bundleId, name, sku, primaryLocale}
Build        {id, version, build, platform, state, uploadedAt, expired, usesNonExemptEncryption}
             state: processing | failed | invalid | valid
Version      {id, platform, version, state, releaseType, earliestReleaseDate, build}
             build: {id, build} or null; state is App Store Connect's appVersionState
Localization {locale, name, subtitle, privacyUrl, privacyChoicesUrl, description, keywords,
              promotionalText, marketingUrl, supportUrl, whatsNew}
Screenshot   {id, fileName, position, checksum, state}
ScreenshotSet{locale, device, displayType, screenshots: [Screenshot]}
Change       {action, scope, locale, field, old, new}     action: create | update | replace | skip
Plan         {changes: [Change]}                           used by push and --dry-run
```

### store apple auth check

Verifies the credentials with one read call.

Result: `{issuerId, keyId, ok}`. Never prints the key.

### store apple app list

All apps of the team. Result: `[App]`.

### store apple app show

The app given by `--app`. Result: `{app: App, versions: [Version], locales: [string]}`.
`versions` holds the newest versions per platform (at most 5).

### store apple build upload

```
store apple build upload <ipa> [--wait] [--timeout 30m] [--notes <text>] [--notes-locale en-US]
                               [--uses-non-exempt-encryption <true|false>]
```

Uploads an IPA through the App Store Connect API. Needs no Xcode, `altool` or Transporter and runs
on Linux.

1. Reads bundle ID, version and build number from `Payload/*.app/Info.plist` inside the IPA.
   `--app` is optional; when given it must match the IPA.
2. Creates a build upload, uploads the file in the parts Apple asks for, commits it with its MD5
   checksum, and waits until Apple accepted the upload (`COMPLETE`). `FAILED` prints Apple's
   errors and exits 1. Apple's warnings are printed to stderr.
3. `--notes`: sets the TestFlight "What to Test" text as soon as the build exists.
4. `--uses-non-exempt-encryption`: sets export compliance as soon as the build exists.
5. `--wait`: additionally waits until the build's processing state is `VALID`.

Result: `{app, appId, version, build, buildUploadId, buildId, state}`. `buildId` is empty when
neither `--wait`, `--notes` nor `--uses-non-exempt-encryption` made the command wait for the build.

```
store apple build upload app.ipa --notes @whats-new.txt --uses-non-exempt-encryption false
```

### store apple build list

```
store apple build list [--version <v>] [--state processing|failed|invalid|valid] [--limit 20]
```

Newest first. Result: `[Build]`.

### store apple build show

```
store apple build show <build> [--version <v>]
```

Result: `Build`.

### store apple build wait

```
store apple build wait <build> [--version <v>] [--timeout 30m]
```

Waits until the build exists and is processed. `VALID` exits 0; `FAILED` or `INVALID` exits 1.
Result: `Build`.

### store apple build set-encryption

```
store apple build set-encryption <build> <true|false> [--version <v>]
```

Sets `usesNonExemptEncryption` (export compliance). Result: `Build`.

### store apple build expire

```
store apple build expire <build> [--version <v>]
```

Expires the build in TestFlight. Result: `Build`.

### store apple testflight notes set

```
store apple testflight notes set <build> --text <text> [--locale en-US] [--version <v>]
```

Sets the "What to Test" text (max 4000 characters). Works while the build is still processing.
Result: `{build, locale, text}`.

### store apple testflight group list

Result: `[{id, name, internal, publicLinkEnabled, publicLink}]`.

### store apple testflight group create

```
store apple testflight group create <name> [--internal] [--public-link] [--public-link-limit <n>]
```

Result: the group.

### store apple testflight group add-build

```
store apple testflight group add-build <group> <build> [--version <v>]
```

`<group>` is the name or ID. External groups need a build that passed Beta App Review (see
`testflight submit`). Result: `{group, build}`.

### store apple testflight tester list

```
store apple testflight tester list [--group <group>]
```

Result: `[{id, email, firstName, lastName, inviteType, state}]`.

### store apple testflight tester add / remove

```
store apple testflight tester add <email> --group <group> [--first-name <s>] [--last-name <s>]
store apple testflight tester remove <email> --group <group>
```

`add` creates the tester if needed and adds them to the group; already a member is a no-op.
`remove` removes them from the group only. Result: the tester.

### store apple testflight submit

```
store apple testflight submit <build> [--version <v>]
```

Submits the build to Beta App Review (needed for external testing). Already submitted is a no-op.
Result: `{build, state}`.

### store apple version list / show

```
store apple version list [--limit 10]
store apple version show <version>
```

Result: `[Version]` / `Version`.

### store apple version create

```
store apple version create <version> [--release manual|after-approval|scheduled]
                                     [--release-date <RFC 3339 time>] [--copyright <text>]
```

Fails if the version exists. `--release scheduled` needs `--release-date`. Result: `Version`.

### store apple version set-build

```
store apple version set-build <version> <build>
```

Attaches a processed (`VALID`) build to an editable version. Result: `Version`.

### store apple version submit

```
store apple version submit <version>
```

Submits the version for App Review. An open, unsubmitted review submission of the app is reused
together with its other items (Apple only accepts e.g. a first subscription group together with an
app version). If that open submission holds a different version, the command fails and names it.
Result: `{version, submissionId, state, items}`.

### store apple version cancel

```
store apple version cancel <version>
```

Withdraws the version's review submission. Result: `{version, submissionId, state}`.

### store apple version release

```
store apple version release <version>
```

Releases a version in `PENDING_DEVELOPER_RELEASE`. Result: `Version`.

### store apple version phased

```
store apple version phased start|pause|resume|complete <version>
```

`start` enables phased release for a version that is not live yet; `pause`, `resume` and
`complete` act on a running one. Result: `{version, state, currentDayNumber}`.

### store apple submit

```
store apple submit <version> [--build N|latest] [--metadata <dir>] [--screenshots <dir>]
                             [--release-notes <dir>] [--release manual|after-approval|scheduled]
                             [--release-date <time>] [--dry-run]
```

The whole App Store submission in one command:

1. Find the build (`--build`, default `latest` = newest `VALID` build of `<version>`).
2. Find or create the version. It must be editable (`PREPARE_FOR_SUBMISSION`,
   `DEVELOPER_REJECTED`, `REJECTED`, `METADATA_REJECTED`, `INVALID_BINARY`); otherwise exit 1
   with its state.
3. Attach the build. Set the release type only when `--release` is given.
4. `--metadata`: run `store apple push <dir>` against this version.
5. `--release-notes`: set "What's New" per locale. While the app has never had a released version,
   this step is skipped with a notice (Apple rejects the field then).
6. Submit as in `version submit`.

Result: `{version, build, submissionId, state, plan}`.

```
store apple submit 1.4.0 --metadata fastlane/metadata --release-notes release-notes --release after-approval
```

### store apple listing show

```
store apple listing show [--locale <l>] [--version <v>]
```

Without `--version`: the editable version if there is one, else the live one. Result:
`[Localization]`.

### store apple listing set

```
store apple listing set --locale <l> [--version <v>] [field flags]
```

Field flags (one per row of the field table below). Only given fields are changed. A locale that
does not exist yet is created. Lengths are checked before any API call. Result: `Localization`.

| File (`push`/`pull`) | Flag | API field | Level | Max chars |
|---|---|---|---|---|
| `name.txt` | `--name` | `name` | app info | 30 |
| `subtitle.txt` | `--subtitle` | `subtitle` | app info | 30 |
| `privacy_url.txt` | `--privacy-url` | `privacyPolicyUrl` | app info | - |
| `privacy_choices_url.txt` | `--privacy-choices-url` | `privacyChoicesUrl` | app info | - |
| `description.txt` | `--description` | `description` | version | 4000 |
| `keywords.txt` | `--keywords` | `keywords` | version | 100 |
| `promotional_text.txt` | `--promotional-text` | `promotionalText` | version | 170 |
| `marketing_url.txt` | `--marketing-url` | `marketingUrl` | version | - |
| `support_url.txt` | `--support-url` | `supportUrl` | version | - |
| `release_notes.txt` | (see `release-notes set`) | `whatsNew` | version | 4000 |

App info fields need an editable app info (not locked by a review); otherwise exit 1 with a hint.

### store apple listing locale add / remove

```
store apple listing locale add <locale> [--version <v>]
store apple listing locale remove <locale> [--version <v>]
```

`add` creates the localization on the version and the app info. `remove` deletes both; removing the
primary locale is refused. Result: `{locale, action}`.

### store apple release-notes set

```
store apple release-notes set <version> (--dir <dir> | --locale <l> --text <text>)
```

Sets "What's New". `--dir` reads `<locale>.txt` per locale. A locale without a localization on the
version is an error that suggests `listing locale add`. Result: `[{locale, whatsNew}]`.

### store apple screenshots list

```
store apple screenshots list [--locale <l>] [--device <d>] [--version <v>]
```

Result: `[ScreenshotSet]`.

### store apple screenshots upload

```
store apple screenshots upload --locale <l> [--device <d>] [--version <v>] [--replace] <png>...
```

- Device from `--device` or from the pixel size of the first file (all files must be the same
  device). Unknown size: exit 1 listing accepted sizes.
- PNG or JPEG, no alpha channel (checked locally, Apple rejects it).
- Without `--replace`: appended to the set (max 10 per set). With `--replace`: if the set already
  holds exactly these files in this order (MD5), nothing happens; otherwise the set is emptied and
  the files are uploaded in order.
- Each file is reserved, uploaded, committed with its MD5 and polled until processed (timeout
  10 minutes per file, `--timeout`).

Result: `ScreenshotSet`.

Devices (`--device` alias, display type, platform). Pixel sizes are in the device table in code;
portrait and landscape are both accepted. Where a size fits several devices, the first row wins.

| Alias | Display type | Platform |
|---|---|---|
| `iphone-6.9` | `APP_IPHONE_67` | ios |
| `iphone-6.5` | `APP_IPHONE_65` | ios |
| `iphone-6.1` | `APP_IPHONE_61` | ios |
| `iphone-5.8` | `APP_IPHONE_58` | ios |
| `iphone-5.5` | `APP_IPHONE_55` | ios |
| `iphone-4.7` | `APP_IPHONE_47` | ios |
| `iphone-4.0` | `APP_IPHONE_40` | ios |
| `iphone-3.5` | `APP_IPHONE_35` | ios |
| `ipad-13` | `APP_IPAD_PRO_3GEN_129` | ios |
| `ipad-12.9` | `APP_IPAD_PRO_129` | ios |
| `ipad-11` | `APP_IPAD_PRO_3GEN_11` | ios |
| `ipad-10.5` | `APP_IPAD_105` | ios |
| `ipad-9.7` | `APP_IPAD_97` | ios |
| `mac` | `APP_DESKTOP` | macos |
| `apple-tv` | `APP_APPLE_TV` | tvos |
| `vision-pro` | `APP_APPLE_VISION_PRO` | visionos |
| `watch-ultra` | `APP_WATCH_ULTRA` | ios |
| `watch-10` | `APP_WATCH_SERIES_10` | ios |
| `watch-7` | `APP_WATCH_SERIES_7` | ios |
| `watch-4` | `APP_WATCH_SERIES_4` | ios |
| `watch-3` | `APP_WATCH_SERIES_3` | ios |

### store apple screenshots delete

```
store apple screenshots delete --locale <l> --device <d> [--version <v>]
```

Deletes the whole set. Result: `{locale, device, deleted}`.

### store apple previews list / upload / delete

```
store apple previews list [--locale <l>] [--device <d>] [--version <v>]
store apple previews upload --locale <l> --device <d> [--version <v>] [--replace]
                            [--poster-frame <timecode>] <video>...
store apple previews delete --locale <l> --device <d> [--version <v>]
```

Same rules as screenshots, but `--device` is required (no size detection for videos), max 3 per
set, and the preview type is the display type without `APP_` (e.g. `IPHONE_67`). Watch devices
have no previews.

### store apple review-info show / set

```
store apple review-info show [--version <v>]
store apple review-info set [--version <v>] [--first-name <s>] [--last-name <s>] [--phone <s>]
                            [--email <s>] [--demo-required <true|false>] [--demo-user <s>]
                            [--demo-password <s>] [--notes <text>] [--attachment <file>]...
```

App Review contact, demo account, notes and attachments for a version. `--attachment` replaces
all attachments with the given files. The demo password is shown as `***` in `show` unless
`--reveal` is given. Result: `{firstName, lastName, phone, email, demoRequired, demoUser,
demoPassword, notes, attachments: [fileName]}`.

### store apple init

```
store apple init <dir> --locale <l> [--locale <l>]... [--screenshots <dir>]
```

Scaffolds the metadata directory for a new app: per locale one empty file per row of the field
table, `review_information/` with its files, and `<screenshots>/<locale>/`. Never touches existing
files, so it can be re-run to add a locale. Works offline, needs no credentials. For an app that
already has a listing, use `pull` instead (an empty file clears its field on `push`).

Result: `{created: [path], existing: [path]}`. Text mode also prints each created file with its
character limit.

### store apple push

```
store apple push <dir> [--screenshots <dir>] [--version <v>]
                       [--only texts,screenshots,review] [--dry-run]
```

Makes the store match the directory (fastlane layout, see "Metadata directory"):

- Target version: `--version`, else the one editable version of `--platform`. None: exit 1 with a
  hint to `version create`.
- A file that exists is the desired value; an empty file clears the field; a missing file leaves
  the field alone.
- A locale folder without a store localization creates it. Store locales without a folder are
  left alone.
- Screenshots: per locale and device, the ordered MD5 list is compared; any difference replaces
  the whole set. Devices without local files are left alone.
- `review_information/` sets the App Review details.
- First the plan is computed and printed (stderr in text mode), then applied. `--dry-run` stops
  after the plan. Unchanged values cause no API call.

Result: `Plan` (with `action` showing what was done).

### store apple pull

```
store apple pull <dir> [--version <v>]
```

Writes the text files of every store locale and `review_information/` (without the demo password)
into `<dir>`. Overwrites the files it writes and touches nothing else. Does not download
screenshots (Apple re-encodes them, so they would never match the originals). Result:
`{locales: [string], files: int}`.

---

## store google

Credentials: `GOOGLE_PLAY_SERVICE_ACCOUNT_JSON` (the service account needs access to the app in
the Play Console).

Every mutating command runs in its own edit: create, change, commit. If anything fails, the edit is
deleted and nothing is published. Mutating commands accept `--changes-not-sent-for-review` (passed
to the commit; needed when the Play Console requires changes to be sent for review manually).

### Result objects

```
Release   {name, versionCodes, status, userFraction, releaseNotes: [{locale, text}]}
Track     {track, releases: [Release]}
Listing   {locale, title, shortDescription, fullDescription, video}
Image     {id, url, sha256}
```

### store google auth check

Creates and deletes an edit for `--app`. Result: `{clientEmail, app, ok}`.

### store google bundle upload

```
store google bundle upload <aab> --track <t> [--track <t>]... [--release-name <s>]
                                 [--status completed|draft|in-progress|halted] [--rollout <0..1>]
                                 [--release-notes <dir>] [--mapping <mapping.txt>]
                                 [--changes-not-sent-for-review]
```

Uploads the AAB (resumable upload), optionally the deobfuscation file for its version code, and
creates one release per `--track` with that version code. `--status` defaults to `completed`;
`--rollout` implies `in-progress`. Apps that were never published only accept `--status draft`;
the error says so. Release notes max 500 characters per locale; locales the listing does not have
are dropped with a warning.

Result: `{versionCode, tracks: [Track]}`.

```
store google bundle upload app.aab --track internal --release-name v1.4.0 --release-notes release-notes
```

### store google track list / show

```
store google track list
store google track show <track>
```

Tracks: `internal`, `alpha`, `beta`, `production`, and custom closed testing tracks.
Result: `[Track]` / `Track`.

### store google track promote

```
store google track promote [<release-name>] --from <t> --to <t> [--rollout <0..1>]
                           [--status completed|draft|in-progress|halted] [--release-notes <dir>]
                           [--changes-not-sent-for-review]
```

Copies a release (by name, default the newest by version code) from one track to another,
`completed` or `in-progress` with `--rollout`. Release notes from `--release-notes`, else the
source release's notes. Already `completed` on the target: no-op. `--status in-progress` and
`halted` need `--rollout`. Result: `Track` (target).

### store google track rollout / halt / resume / complete

```
store google track rollout <track> <fraction>
store google track halt <track>
store google track resume <track>
store google track complete <track>
```

All accept `--changes-not-sent-for-review`. `<fraction>` must be greater than 0 and less than 1.

Act on the track's staged release: change its user fraction, halt it, resume it, or roll it out to
everyone. Result: `Track`.

### store google track testers show / set

```
store google track testers show <track>
store google track testers set <track> --group <google-group-email>...
```

Result: `{track, googleGroups: [string]}`.

### store google release-notes set

```
store google release-notes set <track> [--release-name <s>] (--dir <dir> | --locale <l> --text <text>)
                               [--changes-not-sent-for-review]
```

Sets the release notes of a release on a track (default: the newest by version code). Notes of
other languages stay. Result: `Release`.

### store google listing show / set

```
store google listing show [--locale <l>]
store google listing set --locale <l> [field flags]
```

`set` merges the given fields into the current listing (fields not given are kept). Result:
`[Listing]` / `Listing`.

| File (`push`/`pull`) | Flag | API field | Max chars |
|---|---|---|---|
| `title.txt` | `--title` | `title` | 30 |
| `short_description.txt` | `--short-description` | `shortDescription` | 80 |
| `full_description.txt` | `--full-description` | `fullDescription` | 4000 |
| `video.txt` | `--video` | `video` | - |

### store google images list / upload / delete

```
store google images list --locale <l> [--type <t>]
store google images upload --locale <l> --type <t> [--replace] [--dry-run] [--changes-not-sent-for-review] <png>...
store google images delete --locale <l> --type <t> [--dry-run] [--changes-not-sent-for-review]
```

`--replace` compares the ordered SHA-256 list with Play's and replaces all images of that type
only if it differs. Dimensions and aspect ratio are checked locally before uploading. `list` result:
`[ImageSet]` (`locale`, `type`, `images` with `id`, `url`, `sha256`); `upload`: `ImageSet`; `delete`:
`{locale, type, deleted}`; `--dry-run` of both: the planned change.

| `--type` | Folder or file (`push`/`pull`) | API image type | Count |
|---|---|---|---|
| `phone` | `images/phoneScreenshots/` | `phoneScreenshots` | 2-8 |
| `tablet-7` | `images/sevenInchScreenshots/` | `sevenInchScreenshots` | up to 8 |
| `tablet-10` | `images/tenInchScreenshots/` | `tenInchScreenshots` | up to 8 |
| `tv` | `images/tvScreenshots/` | `tvScreenshots` | up to 8 |
| `wear` | `images/wearScreenshots/` | `wearScreenshots` | up to 8 |
| `feature-graphic` | `images/featureGraphic.png` | `featureGraphic` | 1 |
| `icon` | `images/icon.png` | `icon` | 1 |
| `promo-graphic` | `images/promoGraphic.png` | `promoGraphic` | 1 |
| `tv-banner` | `images/tvBanner.png` | `tvBanner` | 1 |

### store google details show / set

```
store google details show
store google details set [--email <s>] [--website <s>] [--phone <s>] [--default-locale <l>]
```

App contact details. Result: `{email, website, phone, defaultLocale}`.

### store google init

```
store google init <dir> --locale <l> [--locale <l>]...
```

Scaffolds `<dir>/android/<locale>/` per locale: one empty file per row of the field table and the
`images/` folders of the image type table. Same rules as `store apple init` (offline, never touches
existing files). Locales are written as Play codes.

Result: `{created: [path], existing: [path]}`.

### store google push

```
store google push <dir> [--only texts,images] [--dry-run] [--changes-not-sent-for-review]
```

Makes the Play listing match `<dir>/android/` (fastlane layout). Same rules as `store apple push`:
present file = desired value, empty file clears, missing file is left alone, unknown locales are
created, no locale is deleted, plan first, `--dry-run` stops after the plan. Images per locale and
type are replaced only when the ordered SHA-256 list differs. Release notes are not part of
`push`. Result: `Plan`.

### store google pull

```
store google pull <dir>
```

Writes the text files of every Play listing locale into `<dir>/android/<locale>/`. No images in
v1. Result: `{locales: [string], files: int}`.

---

## Metadata directory

fastlane-compatible, so existing fastlane projects work unchanged:

```
<dir>/
  <locale>/                         App Store, e.g. en-US/, de-DE/
    name.txt subtitle.txt privacy_url.txt privacy_choices_url.txt
    description.txt keywords.txt promotional_text.txt marketing_url.txt support_url.txt
    release_notes.txt
  review_information/
    first_name.txt last_name.txt phone_number.txt email_address.txt
    demo_required.txt demo_user.txt demo_password.txt notes.txt
  android/
    <locale>/                       Google Play, e.g. en-US/, de-DE/, es-419/
      title.txt short_description.txt full_description.txt video.txt
      images/phoneScreenshots/*.png  images/sevenInchScreenshots/*.png
      images/tenInchScreenshots/*.png images/tvScreenshots/*.png images/wearScreenshots/*.png
      images/featureGraphic.png images/icon.png images/promoGraphic.png images/tvBanner.png

<screenshots>/                      App Store screenshots, default: <dir>/../screenshots
  <locale>/*.png                    device detected from the pixel size, sorted by file name
  <locale>/<device-alias>/*.png     explicit device, for sizes that fit several devices
```

Release notes for both stores can live in a separate directory with one `<locale>.txt` per locale
(`--release-notes <dir>`).

Start a new directory with `store apple init` / `store google init`, or from an existing listing
with `store apple pull` / `store google pull`.

## Typical CI flow

```
# after the iOS build
store apple build upload app.ipa --notes @notes.txt --uses-non-exempt-encryption false
store apple testflight group add-build "Beta" latest

# after the Android build
store google bundle upload app.aab --track internal --release-name "$TAG" --release-notes release-notes

# release
store apple submit "$VERSION" --metadata fastlane/metadata --release-notes release-notes --release after-approval
store google push fastlane/metadata
store google track promote "$TAG" --from internal --to production --rollout 0.2 --release-notes release-notes
```
