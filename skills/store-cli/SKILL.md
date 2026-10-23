---
name: store-cli
description: Use when working with App Store Connect or Google Play from a terminal, script or CI job - uploading an IPA or AAB, TestFlight groups, testers and notes, App Store submission, review or release, Play tracks, promotion and staged rollouts, store listings, screenshots, release notes - or when the `store` CLI is installed or mentioned.
---

# store CLI

## Overview

`store apple ...` drives App Store Connect, `store google ...` drives Google Play. Commands act on
live store accounts, and several reach users or reviewers at once. Core rule: **every mutating
command runs with `--dry-run` first, and anything that reaches users or reviewers runs only after
the user confirmed the plan the dry run printed.**

`store version` tells whether it is installed. If not: an archive from
https://github.com/nikeee/store-cli/releases, `go install github.com/nikeee/store-cli/cmd/store@latest`,
or in GitHub Actions `uses: nikeee/store-cli@<tag>`.

## Rules

1. **Look up, do not guess.** `store <command> --help` lists every flag, an example and the result
   object. The same text is in `docs/reference/` of https://github.com/nikeee/store-cli.
2. **Dry run first.** Every mutating command takes `--dry-run`: it reads, validates and prints the
   plan, and changes nothing. Show the plan to the user.
3. **Ask before it reaches users or reviewers:** `apple submit`, `apple version submit`, `release`,
   `phased complete`, `apple testflight submit`, `testflight group add-build` (testers get it),
   `google track promote`, `rollout`, `complete`, `bundle upload` to a public track. Ask before
   anything that deletes: `screenshots delete`, `previews delete`, `images delete`, `build expire`,
   `listing locale remove`, `testflight tester remove`.
4. **Credentials come from the environment only:** `APPLE_APPSTORE_ISSUER_ID`,
   `APPLE_APPSTORE_API_KEY_ID`, `APPLE_APPSTORE_API_PRIVATE_KEY`, `GOOGLE_PLAY_SERVICE_ACCOUNT_JSON`.
   Never print, echo, log or write them to a file; never ask for a key in chat. Check them with
   `store apple auth check` / `store google auth check`. When one is missing, name the variable and
   let the user set it.
5. **The app** comes from `--app` (bundle ID / package name) or `APPLE_APPSTORE_BUNDLE_ID` /
   `GOOGLE_PLAY_PACKAGE_NAME`.
6. **Use the CLI, not raw API calls.** It handles tokens, retries, uploads and Play edits.

## Reading results

- stdout is the result, stderr progress and warnings. In an agent session the output is TOON;
  pass `-o json` to parse it with `jq`.
- Exit 0: success, including "nothing to do" (`action: skip`). Exit 1: API error, validation error
  or timeout. Exit 2: usage error.
- Errors carry `error`, `code`, `detail` and often `hint`: the command that fixes it. Dry-run the
  hint before running it.
- "applied N of M changes": a partial failure. `push` and `submit` are declarative, so running
  them again converges; do not patch the rest by hand.

## Quick reference

| Task | Command |
|---|---|
| Upload a build | `store apple build upload app.ipa --wait` (Linux works, no Xcode) / `store google bundle upload app.aab --track internal` |
| Find builds and versions | `store apple build list`, `store apple version list`, `store google track list` |
| Whole App Store submission | `store apple submit 1.4.0 --metadata <dir> --release-notes <dir> --release after-approval` |
| Promote on Play | `store google track promote v1.4.0 --from internal --to production --rollout 0.1` |
| Listing texts and images | `store apple pull <dir>` / `push <dir>`, same for `google` |

Release flows, waiting and CI: [references/releasing.md](references/releasing.md).
Metadata directory, locales, limits and screenshots: [references/listings.md](references/listings.md).

## Common mistakes

| Mistake | Instead |
|---|---|
| Mutating without `--dry-run` because the user said "just release it" | Dry run, show the plan, then ask |
| `echo $APPLE_APPSTORE_API_PRIVATE_KEY` to debug auth | `store apple auth check -v` (logs requests, never secrets) |
| Retrying a failed command unchanged | Read `error` and `hint`; fix the cause |
| Treating `action: skip` as an error | Nothing was needed; exit 0 |
| Expecting `--dry-run` to upload or wait | It uploads nothing; a Play dry run opens and deletes an edit |
