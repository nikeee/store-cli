# store - result objects

Generated from the result types by `go generate ./...`; do not edit.
With `--output json` (and `toon`) a command prints the object named in the `Result:` line of its help,
with exactly these field names, in this order. `[Name]` in a `Result:` line is a list of that object.

## App

| Field | Type |
|---|---|
| `id` | string |
| `bundleId` | string |
| `name` | string |
| `sku` | string |
| `primaryLocale` | string |

## AppDetails

`versions` holds the newest versions per platform (at most 5).

| Field | Type |
|---|---|
| `app` | [App](#app) |
| `versions` | list of [Version](#version) |
| `locales` | list of string |

## AppleAuthCheck

Never contains the key.

| Field | Type |
|---|---|
| `issuerId` | string |
| `keyId` | string |
| `ok` | bool |

## Build

`state` is one of `processing`, `failed`, `invalid`, `valid`.

| Field | Type |
|---|---|
| `id` | string |
| `version` | string |
| `build` | string |
| `platform` | string |
| `state` | string |
| `uploadedAt` | string (RFC 3339 time) |
| `expired` | bool |
| `usesNonExemptEncryption` | bool or null |

## BuildUpload

`buildId` is empty when the command did not wait for the build.

| Field | Type |
|---|---|
| `app` | string |
| `appId` | string |
| `version` | string |
| `build` | string |
| `buildUploadId` | string |
| `buildId` | string |
| `state` | string |

## BuildNotes

| Field | Type |
|---|---|
| `build` | string |
| `locale` | string |
| `text` | string |

## Version

`state` is App Store Connect's appVersionState; `build` is null when no build is attached.

| Field | Type |
|---|---|
| `id` | string |
| `platform` | string |
| `version` | string |
| `state` | string |
| `releaseType` | string |
| `earliestReleaseDate` | string (RFC 3339 time) or null |
| `build` | [VersionBuild](#versionbuild) or null |

## VersionBuild

| Field | Type |
|---|---|
| `id` | string |
| `build` | string |

## Localization

| Field | Type |
|---|---|
| `locale` | string |
| `name` | string |
| `subtitle` | string |
| `privacyUrl` | string |
| `privacyChoicesUrl` | string |
| `description` | string |
| `keywords` | string |
| `promotionalText` | string |
| `marketingUrl` | string |
| `supportUrl` | string |
| `whatsNew` | string |

## LocaleResult

`action` is `create`, `update`, `replace` or `skip` (`skip` when nothing had to change).

| Field | Type |
|---|---|
| `locale` | string |
| `action` | string |

## WhatsNew

| Field | Type |
|---|---|
| `locale` | string |
| `whatsNew` | string |

## BetaGroup

| Field | Type |
|---|---|
| `id` | string |
| `name` | string |
| `internal` | bool |
| `publicLinkEnabled` | bool |
| `publicLink` | string |

## Tester

| Field | Type |
|---|---|
| `id` | string |
| `email` | string |
| `firstName` | string |
| `lastName` | string |
| `inviteType` | string |
| `state` | string |

## TesterRemoved

`action` is `delete`, or `skip` when the tester was not in the group.

| Field | Type |
|---|---|
| `email` | string |
| `group` | string |
| `action` | string |

## GroupBuild

| Field | Type |
|---|---|
| `group` | string |
| `build` | string |

## BetaReview

| Field | Type |
|---|---|
| `build` | string |
| `state` | string |

## SubmissionResult

| Field | Type |
|---|---|
| `version` | string |
| `submissionId` | string |
| `state` | string |
| `items` | int |

## PhasedRelease

| Field | Type |
|---|---|
| `version` | string |
| `state` | string |
| `currentDayNumber` | int |

## SubmitResult

| Field | Type |
|---|---|
| `version` | string |
| `build` | string |
| `submissionId` | string |
| `state` | string |
| `plan` | [Plan](#plan) |

## Screenshot

Also the entry type of app previews.

| Field | Type |
|---|---|
| `id` | string |
| `fileName` | string |
| `position` | int |
| `checksum` | string |
| `state` | string |
| `posterFrame` | string, omitted when empty |

## ScreenshotSet

| Field | Type |
|---|---|
| `locale` | string |
| `device` | string |
| `displayType` | string |
| `screenshots` | list of [Screenshot](#screenshot) |

## ScreenshotsDeleted

`action` is `delete`, or `skip` when there was no set.

| Field | Type |
|---|---|
| `locale` | string |
| `device` | string |
| `action` | string |

## PreviewSet

| Field | Type |
|---|---|
| `locale` | string |
| `device` | string |
| `previewType` | string |
| `previews` | list of [Screenshot](#screenshot) |

## PreviewsDeleted

`action` is `delete`, or `skip` when there was no set.

| Field | Type |
|---|---|
| `locale` | string |
| `device` | string |
| `action` | string |

## ReviewInfo

`demoPassword` is `***` unless `--reveal` is given.

| Field | Type |
|---|---|
| `firstName` | string |
| `lastName` | string |
| `phone` | string |
| `email` | string |
| `demoRequired` | bool |
| `demoUser` | string |
| `demoPassword` | string |
| `notes` | string |
| `attachments` | list of string |

## ApplePullResult

| Field | Type |
|---|---|
| `locales` | list of string |
| `files` | int |

## Plan

Used by `push` and by `--dry-run`.

| Field | Type |
|---|---|
| `changes` | list of [Change](#change) |

## Change

`action` is `create`, `update`, `replace` or `skip`.

| Field | Type |
|---|---|
| `action` | string |
| `scope` | string |
| `locale` | string, omitted when empty |
| `field` | string, omitted when empty |
| `old` | string, omitted when empty |
| `new` | string, omitted when empty |

## InitResult

| Field | Type |
|---|---|
| `created` | list of string |
| `existing` | list of string |

## LintResult

`errors` and `warnings` count the findings.

| Field | Type |
|---|---|
| `findings` | list of [Finding](#finding) |
| `errors` | int |
| `warnings` | int |

## Finding

`severity` is `error` or `warning`; `file`, `line` and `col` are left out when unknown.

| Field | Type |
|---|---|
| `severity` | string |
| `file` | string, omitted when empty |
| `line` | int, omitted when empty |
| `col` | int, omitted when empty |
| `message` | string |

## VersionInfo

`dev` for local builds.

| Field | Type |
|---|---|
| `version` | string |
| `commit` | string |
| `date` | string |

## GoogleAuthCheck

| Field | Type |
|---|---|
| `clientEmail` | string |
| `app` | string |
| `ok` | bool |

## BundleUploadResult

| Field | Type |
|---|---|
| `versionCode` | int |
| `tracks` | list of [Track](#track) |

## Track

| Field | Type |
|---|---|
| `track` | string |
| `releases` | list of [Release](#release) |

## Release

| Field | Type |
|---|---|
| `name` | string |
| `versionCodes` | list of string |
| `status` | string |
| `userFraction` | number, omitted when empty |
| `releaseNotes` | list of [ReleaseNote](#releasenote) |

## ReleaseNote

| Field | Type |
|---|---|
| `locale` | string |
| `text` | string |

## Testers

| Field | Type |
|---|---|
| `track` | string |
| `googleGroups` | list of string |

## Listing

| Field | Type |
|---|---|
| `locale` | string |
| `title` | string |
| `shortDescription` | string |
| `fullDescription` | string |
| `video` | string |

## Details

| Field | Type |
|---|---|
| `email` | string |
| `website` | string |
| `phone` | string |
| `defaultLocale` | string |

## Image

| Field | Type |
|---|---|
| `id` | string |
| `url` | string |
| `sha256` | string |

## ImageSet

| Field | Type |
|---|---|
| `locale` | string |
| `type` | string |
| `images` | list of [Image](#image) |

## ImagesDeleted

`action` is `delete`, or `skip` when there were no images; `deleted` is the number of removed images.

| Field | Type |
|---|---|
| `locale` | string |
| `type` | string |
| `action` | string |
| `deleted` | int |

## GooglePullResult

| Field | Type |
|---|---|
| `locales` | list of string |
| `files` | int |
