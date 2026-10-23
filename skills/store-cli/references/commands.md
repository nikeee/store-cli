# store commands

Generated from the command help by `go generate ./...`; do not edit.
`store <command> --help` shows the flags, an example and the result of each command.

## store apple

- `store apple app list` - List all apps of the team
- `store apple app show` - Show the app given by --app
- `store apple auth check` - Verify the credentials with one read call
- `store apple build expire <build>` - Expire a build in TestFlight
- `store apple build list` - List builds, newest first
- `store apple build set-encryption <build> <true|false>` - Set the export compliance answer of a build
- `store apple build show <build>` - Show one build
- `store apple build upload <ipa>` - Upload an IPA to App Store Connect
- `store apple build wait <build>` - Wait until a build exists and is processed
- `store apple init <dir> --locale <l> [--locale <l>]... [--screenshots <dir>]` - Scaffold the metadata directory for a new app
- `store apple lint <dir> [--screenshots <dir>] [--only texts,screenshots,review] [--release-notes <dir>] [--strict]` - Check a metadata directory offline
- `store apple listing locale add <locale>` - Add a locale to the version and the app info
- `store apple listing locale remove <locale>` - Remove a locale from the version and the app info
- `store apple listing set --locale <l> [--version <v>] [field flags]` - Change listing texts of one locale
- `store apple listing show` - Show the listing texts per locale
- `store apple previews delete --locale <l> --device <d> [--version <v>]` - Delete the app preview set of one locale and device
- `store apple previews list [--locale <l>] [--device <d>] [--version <v>]` - List the app preview sets of a version
- `store apple previews upload --locale <l> --device <d> [--version <v>] [--replace] [--poster-frame <timecode>] <video>...` - Upload app previews to the set of one locale and device
- `store apple pull <dir> [--version <v>]` - Write the store listing into a metadata directory
- `store apple push <dir> [--screenshots <dir>] [--version <v>] [--only texts,screenshots,review]` - Make App Store Connect match a metadata directory
- `store apple release-notes set <version> (--dir <dir> | --locale <l> --text <text>)` - Set What's New of a version
- `store apple review-info set [--version <v>] [field flags] [--attachment <file>]...` - Change the App Review information
- `store apple review-info show [--version <v>]` - Show the App Review information
- `store apple screenshots delete --locale <l> --device <d> [--version <v>]` - Delete the screenshot set of one locale and device
- `store apple screenshots list [--locale <l>] [--device <d>] [--version <v>]` - List the screenshot sets of a version
- `store apple screenshots upload --locale <l> [--device <d>] [--version <v>] [--replace] <png>...` - Upload screenshots to the set of one locale and device
- `store apple submit <version> [--build N|latest] [--metadata <dir>] [--screenshots <dir>] [--release-notes <dir>] [--release manual|after-approval|scheduled] [--release-date <time>]` - Do the whole App Store submission of a version
- `store apple testflight group add-build <group> <build>` - Add a build to a beta group
- `store apple testflight group create <name>` - Create a beta group
- `store apple testflight group list` - List the beta groups of the app
- `store apple testflight notes set <build>` - Set the What to Test text of a build
- `store apple testflight submit <build>` - Submit a build to Beta App Review
- `store apple testflight tester add <email>` - Add a tester to a beta group
- `store apple testflight tester list` - List the testers of the app
- `store apple testflight tester remove <email>` - Remove a tester from a beta group
- `store apple version cancel <version>` - Withdraw the review submission of a version
- `store apple version create <version>` - Create a new App Store version
- `store apple version list` - List the versions of the app, newest first
- `store apple version phased complete <version>` - Release to all users now, ending the phased release
- `store apple version phased pause <version>` - Pause the running phased release
- `store apple version phased resume <version>` - Resume a paused phased release
- `store apple version phased start <version>` - Enable phased release for a version
- `store apple version release <version>` - Release a version that waits for the developer release
- `store apple version set-build <version> <build>` - Attach a build to an editable version
- `store apple version show <version>` - Show one version
- `store apple version submit <version>` - Submit a version for App Review

## store google

- `store google auth check` - Verify the credentials by creating and deleting an edit
- `store google bundle upload <aab>` - Upload an AAB and release it on tracks
- `store google details set [--email <s>] [--website <s>] [--phone <s>] [--default-locale <l>]` - Change the app contact details
- `store google details show` - Show the app contact details
- `store google images delete --locale <l> --type <t>` - Delete all images of one locale and type
- `store google images list --locale <l> [--type <t>]` - List the images of a locale
- `store google images upload --locale <l> --type <t> [--replace] <image>...` - Upload images of one locale and type
- `store google init <dir> --locale <l> [--locale <l>]...` - Scaffold the Play listing directory for a new app
- `store google lint <dir> [--only texts,images] [--release-notes <dir>] [--strict]` - Check a metadata directory offline
- `store google listing set --locale <l> [field flags]` - Change listing texts of one locale
- `store google listing show` - Show the listing texts per locale
- `store google pull <dir>` - Write the Play listing into a metadata directory
- `store google push <dir> [--only texts,images]` - Make the Play listing match a metadata directory
- `store google release-notes set <track> (--dir <dir> | --locale <l> --text <text>)` - Set the release notes of a release on a track
- `store google track complete <track>` - Roll the staged release of a track out to everyone
- `store google track halt <track>` - Halt the staged rollout of a track
- `store google track list` - List the tracks and their releases
- `store google track promote [<release-name>] --from <track> --to <track>` - Copy a release from one track to another
- `store google track resume <track>` - Resume the halted rollout of a track
- `store google track rollout <track> <fraction>` - Change the rollout fraction of a track
- `store google track show <track>` - Show a track and its releases
- `store google track testers set <track> --group <google-group-email>...` - Replace the tester groups of a track
- `store google track testers show <track>` - Show the tester groups of a track

## store version

- `store version` - Print the version
