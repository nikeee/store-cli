# store lint - design

Issue: #45. Builds on commit 5583514 (GitHub Actions annotations, `clierr.Issue`, `clierr.Collector`).

## Intent

A metadata directory can be broken in ways that only show when `push` runs, when App Review
rejects the version, or never (the listing is just worse than it could be). `lint` checks the
directory offline and reports every problem in one run: what the store rejects, what is missing to
submit or publish, and what Apple and Google recommend.

It serves two uses equally (user decision):

- **Locally**, before `push`: a readable list of what to fix.
- **In CI**, as a step that stops the build on errors, with every finding shown as a GitHub
  annotation on the file in the PR.

## Goals

- One command per store, mirroring `push`: `store apple lint <dir>` and `store google lint <dir>`.
- Same inputs and flags as `push` of that store (`--screenshots`, `--platform` on Apple, `--only`
  with the same parts), plus `--release-notes <dir>` and `--strict` (below).
- Offline: no credentials, no network. Works where `push` would need credentials.
- All findings in one run. Nothing stops at the first problem, including problems that today make
  `ReadPushDir` or the directory scans return early (unknown device folder, an image whose size fits
  no device, an unreadable image).
- Every finding has a severity, a file (when it is about one), line and column (when known) and a
  message.
- Reuse the checks `push` already does. One rule lives in one place: every check of `push` is also
  a lint error, from the same code. Lint is stricter than `push` on purpose (see "Split between push
  and lint").

## Non-goals

- No auto-fixes (re-encoding, stripping alpha, resizing, trimming texts). Lint only reports.
- No checks against the live listing or the API.
- No combined `store lint` for both stores.
- No per-locale "screenshots missing" findings: both stores fall back to the default language.
- A missing file is not a finding. For `push` it means "leave the field alone", so lint cannot know
  the field is empty in the store. Only an existing empty file counts as empty.
- No check for previews, in-app purchases, or anything `push` does not handle.

## Anti-goals

- A noisy linter. A warning that fires on a typical, healthy directory is a bug.
- `lint` passing while `push` of the same directory fails on validation. The reverse is intended:
  lint also reports what blocks submitting or publishing, which `push` does not refuse.

## Behavior

### Severities

- `error`: the store rejects it, `push` fails on it, or the store requires it to submit (Apple) or
  publish (Google).
- `warning`: accepted, but against a recommendation from Apple or Google, or likely a mistake.

### Exit code

- 0: no errors (warnings allowed).
- 1: at least one error, or with `--strict` at least one warning. Also when the directory does not
  exist (same message as `push`).
- 2: usage error (bad flag, bad `--only` part).

### Output

The result goes to stdout as `LintResult` (registered in `tools/gendocs/results.go`, named in the
`Result:` line):

```json
{"findings": [{"severity": "error", "file": "fastlane/metadata/de-DE/keywords.txt", "line": 1, "col": 87,
               "message": "104 bytes, limit 100"}],
 "errors": 1, "warnings": 0}
```

`file`, `line`, `col` are omitted or 0 when unknown, with the same meaning as `clierr.Issue`.
Findings are sorted by file, then line, then col; file-less findings come first.

Text mode prints one line per finding (severity, `file:line:col`, message), then a summary line
(`2 errors, 1 warning`, or `no problems`). Severity is colored like other output.

With `GITHUB_ACTIONS=true` (the printer's existing switch), every finding becomes exactly one
annotation on stderr: `::error` or `::warning` with `file`, `line`, `col` as known. The failure that
sets exit code 1 must not annotate the findings a second time and must not drop them (today
`Printer.Error` annotates every `clierr.Issue` of the error it prints, `Printer.Warnf` has no file).
How that is arranged is left to the builder; a likely shape is a `Printer` method that annotates one
`clierr.Issue` at a given level, used for warnings, with the error findings carried as the issues of
the final error.

The command's help explains each check in one line per rule (it becomes the generated reference).
`docs/commands.md` gets lint in the typical CI flow, before `push`.

## Checks

"Today" means `push` already checks it; those findings come from the existing validators, now
collected instead of returned at the first problem.

### Apple (`store apple lint`)

Errors:

1. Today: text limits of every field in `<locale>/` and `review_information/`; empty `name.txt`;
   `demo_required.txt` not `true`/`false`; screenshot format (PNG/JPEG), alpha channel, pixel size
   not accepted by the device, more than 10 per set; unknown device folder; image size that fits no
   device of `--platform`.
2. Name shorter than 2 characters (Apple: "at least two characters and no more than 30").
3. Keywords over 100 **bytes** (UTF-8). Apple: "You can provide up to 100 bytes of content." Today
   the limit is counted in runes, so `ä` counts 1 instead of 2. Fix it in the field table so `push`,
   `listing set` and `lint` all count bytes for keywords; the generated tables show `100 bytes` for
   that field. Other fields stay in characters.
5. Empty file for a required field: `description.txt`, `keywords.txt`, `support_url.txt`
   (Apple marks each "This property is required").
6. Apple Watch screenshots of more than one pixel size across all locales. Apple: "You must use the
   same screenshot size for Apple Watch consistently across all localizations for the app."

Warnings:

4. A keyword (comma separated, trimmed) of 2 characters or fewer. Apple: each keyword must be
   "greater than two characters", but short terms are normal in CJK locales (`写真`) and as acronyms
   (`AI`, `3D`), so it is a warning (user decision). Position: the keyword's place in the file.
7. With `--platform ios` and at least one screenshot in the directory: no `iphone-6.9` and no
   `iphone-6.5` set in any locale. Apple requires one of them if the app runs on iPhone; offline
   lint cannot know that, hence a warning.
8. At least one iPad set in any locale, but no `ipad-13` set in any locale. Apple: "Required if app
   runs on iPad."
9. Keywords: a space after a comma (wastes bytes; Apple: "separated by commas and no spaces"), a
   term repeated (case-insensitive), a word that also appears in the same locale's `name.txt` or
   `subtitle.txt` (Apple: "Don't repeat words from the app name, subtitle"; the app is searchable by
   name already), the word "app". Position: the place of the offending part in `keywords.txt`.

### Google (`store google lint`)

Errors:

10. Today: text limits; a locale folder that has to be renamed to its Play code; image format, alpha
    channel, exact sizes, side limits, aspect ratio; counts per type (phone 2 to 8, others up to 8,
    single-image types exactly one).
11. `images/icon.png` larger than 1024 KB (Play: "maximum 1024 KB").

Warnings:

12. With at least one image in the directory: no feature graphic in any locale (Play: "You must
    provide a feature graphic to publish"); fewer than 2 screenshots in every locale, counting all
    screenshot types together (Play: "a minimum of two screenshots across different device types").
    A warning, not an error, because a repo may keep only some images and the rest in Play Console
    (user decision).
13. A phone screenshot set with fewer than 4 screenshots (Play's conditions to be eligible for
    promotion, "Highly recommended", also for 14).
14. A phone screenshot with a side under 1080 px, or not 9:16 / 16:9.
15. `title.txt` with an emoji or emoticon, all letters upper case, a repeated special character
    (like `!!`), or a ranking or promotion term: `#1`, `best`, `free`, `% off` (case-insensitive,
    whole word). Play metadata policy: "Don't use emojis, emoticons, or repeated special
    characters", "Avoid ALL CAPS unless it is part of your brand name", no text that indicates
    "store performance or ranking" or promotion.

### Both stores

16. `--release-notes <dir>`: the release notes directory is read with the store's reader
    (`apple.ReadReleaseNotesDir`, limit 4000; `google.ReadReleaseNotesDir`, limit 500) and its
    problems are errors. Without the flag nothing is read. (Builder's call, see below.)

### Sources

Retrieved 2026-10-23. Cite them in code comments next to the rule constants, like
`internal/media/devices.go` does.

- Apple screenshot specifications: https://developer.apple.com/help/app-store-connect/reference/screenshot-specifications
- Apple platform version information (description, keywords, support URL):
  https://developer.apple.com/help/app-store-connect/reference/app-information/platform-version-information/
- Apple app information (name): https://developer.apple.com/help/app-store-connect/reference/app-information/app-information/
- Apple App Store search (keyword format and repetition): https://developer.apple.com/app-store/search/
- Play preview assets: https://support.google.com/googleplay/android-developer/answer/9866151
- Play metadata policy: https://support.google.com/googleplay/android-developer/answer/9898842

## Constraints

- Go, existing packages and idioms: `clierr.Issue` and `clierr.Collector` for findings with
  positions, `media` for image facts, `metadata` for field tables, cobra commands in `internal/cli`.
- Rule data (limits, word lists, device facts) lives once, in the existing tables where one exists
  (`metadata` fields, `media.Devices`, `media.ImageTypes`).
- Tests offline with the existing helpers (`internal/testutil/pngfake`, golden files); every rule
  gets a test that fires and one that stays quiet on a healthy directory.
- `go generate ./...` keeps the reference, `tables.md`, `results.md` and the skill's command list in
  sync.
- No en or em dashes anywhere.

## Decided by the user

- Lint works locally and in CI equally.
- Recommendations from Apple and Google are part of it, not only hard rules.
- The check catalog above (rules 1 to 15) and the out-of-scope list. Rule numbers are ids, not
  order.
- Rules 4 and 12 are warnings, not errors: they could block healthy setups in CI for good.
  `--strict` enforces them.
- Build on the workflow-commands work (5583514) instead of a separate finding format.

## Split between push and lint

- Rules where the store's API rejects the value join the shared validation, so `push` (and
  `listing set` for texts) fail before the API call instead of at it: 2 (name length), 3 (keyword
  bytes), 11 (icon file size). They are lint errors through that shared code.
- Rules about being ready to submit or publish (5, 6) and all warnings are lint-only. `push`
  keeps its documented behavior: an empty file clears the field, a missing image type is left
  alone.

## Builder's calls (made while writing this, not discussed)

- The split between push and lint above.
- GitHub shows only a limited number of annotations per step and per job and drops the rest. That
  is accepted: the full list is always in the step log (stdout). No job summary. Warnings are
  annotated by the command, errors by the failure that sets exit code 1.
- `--strict` turns warnings into exit code 1.
- `--release-notes <dir>` (rule 16) is included because the typical CI flow passes such a directory
  to `submit`, `bundle upload` and `track promote`, and its limit is not checked by `push`.
- Rule 12 counts screenshots of all types instead of only phone screenshots, matching Play's wording.
- Presence rules (7, 8, 12) run only when the directory holds at least one image, so a directory
  that does not manage images stays quiet.
- Rule 15 checks English terms only, in every locale.

## Left to the builder

- Package layout: where the rules live (per store next to `ReadPushDir`, or a shared package) and
  how `ReadPushDir`'s validation and the scans are reshaped to collect instead of stop, without
  changing what `push` accepts.
- The `LintResult` and finding type names and where they live.
- How a file-less directory problem (unknown device folder, Play folder rename) is shown; giving it
  the folder as file is fine if annotations still render.
- Exact emoji and "repeated special character" detection, as long as it stays quiet on normal
  punctuation (`...` in a title may be fine; decide and test).
- Exact wording of messages, following the existing style (lower case, no trailing period, names
  the limit).
