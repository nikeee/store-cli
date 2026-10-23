# Store listings with store

The listing lives in a fastlane-compatible directory. `push` makes the store match it, `pull`
writes the store into it. Field files, devices, image types and the locale mapping are listed in
`docs/reference/tables.md` (https://github.com/nikeee/store-cli).

```
<dir>/
  <locale>/*.txt                 App Store, App Store codes: en-US, de-DE, it, zh-Hans
  review_information/*.txt       App Review contact, demo account, notes
  android/<language>/*.txt       Google Play, Play codes: en-US, it-IT, es-419, zh-CN
  android/<language>/images/     phoneScreenshots/, featureGraphic.png, icon.png, ...
<screenshots>/<locale>/*.png     App Store screenshots, default <dir>/../screenshots
```

## Workflow

```sh
store apple pull fastlane/metadata                 # start from the live listing (or: init --locale ...)
# edit the .txt files
store apple lint fastlane/metadata                 # offline: every problem and store recommendation
store apple push fastlane/metadata --dry-run       # plan: create / update / delete / skip per field
store apple push fastlane/metadata                 # after the user agreed to the plan
```

Same with `store google pull|lint|push`. `--only texts,screenshots,review` (Apple) or
`--only texts,images` (Google) limits a push. `store apple submit --metadata <dir>` runs the push
as part of a submission.

## What a file means

- **Missing file:** the field is left alone.
- **Empty file:** the field is cleared. `name.txt` cannot be empty; an empty `demo_required.txt`
  changes nothing.
- Trailing whitespace is trimmed on both sides of the comparison.
- `init` creates empty files: fill them or delete them before a push, or the push clears fields.

## Checks before any request

Lengths, locale names, image sizes, alpha channels and counts are checked locally, all problems
in one error, exit 1. Fix the files; nothing was sent. `lint` runs the same checks without credentials and adds
what blocks a submission (empty required files) and warnings for store recommendations.

- Apple screenshots: PNG without alpha or JPEG. The device comes from the pixel size; for a size
  that fits several devices put the files in `<locale>/<device-alias>/`. Order is the file name
  order. One changed file replaces the whole set of that device; other devices stay.
- Google: folders under `android/` use Play codes; an App Store code (`zh-Hans`) is an error that
  names the Play code. Phone screenshots 2 to 8, feature graphic exactly 1024x500.
- App Store app info texts (name, subtitle, privacy URL) change only while an app info is
  editable; version texts only on an editable version (`--version`, default the one editable).

## Single changes

For one field or one image set, use the direct commands instead of a push:
`store apple listing set --locale de-DE --description @desc.txt`,
`store apple screenshots upload --locale en-US --replace shots/*.png`,
`store google listing set --locale de-DE --title "..."`,
`store google images upload --locale en-US --type feature-graphic feature.png`.
Text flags take a value or `@file`.

## Review information

`review_information/demo_password.txt` is never printed: output shows `***`
(`review-info show --reveal` to see it). Do not paste it into chat or logs.
