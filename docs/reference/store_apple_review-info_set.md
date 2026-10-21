## store apple review-info set

Change the App Review information

### Synopsis

Changes the given fields; fields not given stay as they are. An empty value clears a field, except
--demo-required, which cannot be cleared. A text starting with @ is read from that file.
--attachment (repeatable) replaces all existing attachments with the given files. The
password is masked as in show; with --dry-run the result is the planned Plan.

Result: ReviewInfo

```
store apple review-info set [--version <v>] [field flags] [--attachment <file>]... [flags]
```

### Examples

```
  store apple review-info set --demo-required true --demo-user reviewer --demo-password @pw.txt
  store apple review-info set --attachment login-flow.pdf
```

### Options

```
      --attachment stringArray   attachment file for App Review; replaces all existing attachments (repeatable)
      --demo-password string     Demo account password, or @file to read it from a file
      --demo-required string     Whether a demo account is needed: true or false, or @file to read it from a file
      --demo-user string         Demo account user name, or @file to read it from a file
      --dry-run                  print what would change, change nothing
      --email string             Review contact email address, or @file to read it from a file
      --first-name string        Review contact first name, or @file to read it from a file
  -h, --help                     help for set
      --last-name string         Review contact last name, or @file to read it from a file
      --notes string             Notes for the App Review team, or @file to read it from a file
      --phone string             Review contact phone number, or @file to read it from a file
      --version string           app version (default: the editable version, else the live one)
```

### Options inherited from parent commands

```
      --app string        bundle ID of the app, e.g. com.example.app (default $APPLE_APPSTORE_BUNDLE_ID)
  -o, --output string     output format: text, json or toon (default: auto)
      --platform string   platform of versions, builds and screenshots: ios, macos, tvos or visionos (default "ios")
  -v, --verbose           log every HTTP request to stderr
```

### SEE ALSO

* [store apple review-info](store_apple_review-info.md)	 - Show or change the App Review information

