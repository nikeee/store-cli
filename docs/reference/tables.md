# store - tables

Generated from the code tables by `go generate ./...`; do not edit.

## Environment variables

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
| `GITHUB_ACTIONS` | all | `true` (set by GitHub Actions): errors and warnings also become annotations |
| `GITHUB_WORKSPACE` | all | Set by GitHub Actions: annotations name files relative to it |

## Agent markers

If any of these environment variables is set, the default output format is `toon` instead of `text`.

| Variable |
|---|
| `AI_AGENT` |
| `CLAUDECODE` |
| `CURSOR_AGENT` |
| `GEMINI_CLI` |
| `CODEX_SANDBOX` |
| `CODEX_THREAD_ID` |

## App Store listing fields

Per locale. Files live in `<dir>/<locale>/`; flags belong to `store apple listing set`. `app-info` fields need an editable app info.

| File | Flag | API field | Level | Max chars |
|---|---|---|---|---|
| `name.txt` | `--name` | `name` | app-info | 30 |
| `subtitle.txt` | `--subtitle` | `subtitle` | app-info | 30 |
| `privacy_url.txt` | `--privacy-url` | `privacyPolicyUrl` | app-info | - |
| `privacy_choices_url.txt` | `--privacy-choices-url` | `privacyChoicesUrl` | app-info | - |
| `description.txt` | `--description` | `description` | version | 4000 |
| `keywords.txt` | `--keywords` | `keywords` | version | 100 |
| `promotional_text.txt` | `--promotional-text` | `promotionalText` | version | 170 |
| `marketing_url.txt` | `--marketing-url` | `marketingUrl` | version | - |
| `support_url.txt` | `--support-url` | `supportUrl` | version | - |
| `release_notes.txt` | - | `whatsNew` | version | 4000 |

## App Review information fields

Files live in `<dir>/review_information/`; flags belong to `store apple review-info set`. The demo password is masked in output.

| File | Flag | API field | Level | Max chars |
|---|---|---|---|---|
| `first_name.txt` | `--first-name` | `contactFirstName` | review | - |
| `last_name.txt` | `--last-name` | `contactLastName` | review | - |
| `phone_number.txt` | `--phone` | `contactPhone` | review | - |
| `email_address.txt` | `--email` | `contactEmail` | review | - |
| `demo_required.txt` | `--demo-required` | `demoAccountRequired` | review | - |
| `demo_user.txt` | `--demo-user` | `demoAccountName` | review | - |
| `demo_password.txt` | `--demo-password` | `demoAccountPassword` | review | - |
| `notes.txt` | `--notes` | `notes` | review | 4000 |

## Google Play listing fields

Per locale. Files live in `<dir>/android/<locale>/`; flags belong to `store google listing set`.

| File | Flag | API field | Level | Max chars |
|---|---|---|---|---|
| `title.txt` | `--title` | `title` | listing | 30 |
| `short_description.txt` | `--short-description` | `shortDescription` | listing | 80 |
| `full_description.txt` | `--full-description` | `fullDescription` | listing | 4000 |
| `video.txt` | `--video` | `video` | listing | - |

## App Store screenshot and preview devices

`--device` aliases. Sizes are portrait width x height pixels; landscape is accepted too. Where a size fits several devices of a platform, the first row wins. Watch devices have no previews.

| Alias | Display type | Preview type | Platform | Sizes |
|---|---|---|---|---|
| `iphone-6.9` | `APP_IPHONE_67` | `IPHONE_67` | ios | 1260x2736, 1290x2796, 1320x2868 |
| `iphone-6.5` | `APP_IPHONE_65` | `IPHONE_65` | ios | 1284x2778, 1242x2688 |
| `iphone-6.1` | `APP_IPHONE_61` | `IPHONE_61` | ios | 1179x2556, 1206x2622 |
| `iphone-5.8` | `APP_IPHONE_58` | `IPHONE_58` | ios | 1170x2532, 1125x2436, 1080x2340 |
| `iphone-5.5` | `APP_IPHONE_55` | `IPHONE_55` | ios | 1242x2208 |
| `iphone-4.7` | `APP_IPHONE_47` | `IPHONE_47` | ios | 750x1334 |
| `iphone-4.0` | `APP_IPHONE_40` | `IPHONE_40` | ios | 640x1096, 640x1136 |
| `iphone-3.5` | `APP_IPHONE_35` | `IPHONE_35` | ios | 640x920, 640x960 |
| `ipad-13` | `APP_IPAD_PRO_3GEN_129` | `IPAD_PRO_3GEN_129` | ios | 2064x2752, 2048x2732 |
| `ipad-12.9` | `APP_IPAD_PRO_129` | `IPAD_PRO_129` | ios | 2048x2732 |
| `ipad-11` | `APP_IPAD_PRO_3GEN_11` | `IPAD_PRO_3GEN_11` | ios | 1488x2266, 1668x2420, 1668x2388, 1640x2360 |
| `ipad-10.5` | `APP_IPAD_105` | `IPAD_105` | ios | 1668x2224 |
| `ipad-9.7` | `APP_IPAD_97` | `IPAD_97` | ios | 1536x2008, 1536x2048, 768x1004, 768x1024 |
| `mac` | `APP_DESKTOP` | `DESKTOP` | macos | 800x1280, 900x1440, 1600x2560, 1800x2880 |
| `apple-tv` | `APP_APPLE_TV` | `APPLE_TV` | tvos | 1080x1920, 2160x3840 |
| `vision-pro` | `APP_APPLE_VISION_PRO` | `APPLE_VISION_PRO` | visionos | 2160x3840 |
| `watch-ultra` | `APP_WATCH_ULTRA` | - | ios | 422x514, 410x502 |
| `watch-10` | `APP_WATCH_SERIES_10` | - | ios | 416x496 |
| `watch-7` | `APP_WATCH_SERIES_7` | - | ios | 396x484 |
| `watch-4` | `APP_WATCH_SERIES_4` | - | ios | 368x448 |
| `watch-3` | `APP_WATCH_SERIES_3` | - | ios | 312x390 |

## Google Play image types

`--type` aliases. Paths are relative to `<dir>/android/<locale>/`; a folder holds several images in file name order.

| `--type` | Folder or file | API image type | Count | Size | Alpha |
|---|---|---|---|---|---|
| `phone` | `images/phoneScreenshots` | `phoneScreenshots` | 2 to 8 | 320-3840 px per side, long side at most 2x the short side | no |
| `tablet-7` | `images/sevenInchScreenshots` | `sevenInchScreenshots` | up to 8 | 320-3840 px per side, long side at most 2x the short side | no |
| `tablet-10` | `images/tenInchScreenshots` | `tenInchScreenshots` | up to 8 | 320-3840 px per side, long side at most 2x the short side | no |
| `tv` | `images/tvScreenshots` | `tvScreenshots` | up to 8 | 320-3840 px per side, long side at most 2x the short side | no |
| `wear` | `images/wearScreenshots` | `wearScreenshots` | up to 8 | square, 384-3840 px per side | no |
| `feature-graphic` | `images/featureGraphic.png` | `featureGraphic` | 1 | 1024x500 | no |
| `icon` | `images/icon.png` | `icon` | 1 | 512x512 | yes |
| `promo-graphic` | `images/promoGraphic.png` | `promoGraphic` | 1 | 180x120 | no |
| `tv-banner` | `images/tvBanner.png` | `tvBanner` | 1 | 1280x720 | no |

## Locale mapping

App Store locales whose Google Play language code differs. Every other locale is the same in both stores. The Google commands take either form and write Play codes.

| App Store | Google Play |
|---|---|
| `ar-SA` | `ar` |
| `cs` | `cs-CZ` |
| `da` | `da-DK` |
| `el` | `el-GR` |
| `es-MX` | `es-419` |
| `fi` | `fi-FI` |
| `gu-IN` | `gu` |
| `he` | `iw-IL` |
| `hi` | `hi-IN` |
| `hu` | `hu-HU` |
| `it` | `it-IT` |
| `ja` | `ja-JP` |
| `ko` | `ko-KR` |
| `no` | `no-NO` |
| `pa-IN` | `pa` |
| `pl` | `pl-PL` |
| `ru` | `ru-RU` |
| `sl-SI` | `sl` |
| `sv` | `sv-SE` |
| `tr` | `tr-TR` |
| `ur-PK` | `ur` |
| `zh-Hans` | `zh-CN` |
| `zh-Hant` | `zh-TW` |
