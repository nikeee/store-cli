## store google details set

Change the app contact details

### Synopsis

Changes the given fields; fields not given stay as they are. An empty value clears a field. A text
starting with @ is read from that file. --default-locale takes an App Store style locale and is
sent as the Play language code. Result: {email, website, phone, defaultLocale}; with --dry-run
the details as they would be.

```
store google details set [--email <s>] [--website <s>] [--phone <s>] [--default-locale <l>] [flags]
```

### Examples

```
  store google details set --email support@example.com --website https://example.com
  store google details set --default-locale en-US
```

### Options

```
      --changes-not-sent-for-review   commit without sending the changes for review; needed when Play Console requires sending them manually
      --default-locale string         default language of the listing, e.g. en-US
      --dry-run                       print what would change, change nothing
      --email string                  contact email address, or @file to read it from a file
  -h, --help                          help for set
      --phone string                  contact phone number, or @file to read it from a file
      --website string                contact website, or @file to read it from a file
```

### Options inherited from parent commands

```
      --app string      package name, e.g. com.example.app (default $GOOGLE_PLAY_PACKAGE_NAME)
  -o, --output string   output format: text, json or toon (default: auto)
  -v, --verbose         log every HTTP request to stderr
```

### SEE ALSO

* [store google details](store_google_details.md)	 - Show or change the app contact details

