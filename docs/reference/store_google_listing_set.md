## store google listing set

Change listing texts of one locale

### Synopsis

Changes the given fields of one locale; fields not given stay as they are (including the promo video).
A locale that does not exist yet is created. An empty value clears a field. Lengths are checked
before any API call. --locale takes an App Store style locale, sent as the Play language code. A text
starting with @ is read from that file. With --dry-run the listing is shown as it would be.

Result: Listing

```
store google listing set --locale <l> [field flags] [flags]
```

### Examples

```
  store google listing set --locale en-US --short-description "Fast and small" --full-description @full.txt
```

### Options

```
      --changes-not-sent-for-review   commit without sending the changes for review; needed when Play Console requires sending them manually
      --dry-run                       print what would change, change nothing
      --full-description string       Full description, or @file to read it from a file
  -h, --help                          help for set
      --locale string                 locale to change, e.g. en-US (required)
      --short-description string      Short description, or @file to read it from a file
      --title string                  App title, or @file to read it from a file
      --video string                  Promo video URL, or @file to read it from a file
```

### Options inherited from parent commands

```
      --app string      package name, e.g. com.example.app (default $GOOGLE_PLAY_PACKAGE_NAME)
  -o, --output string   output format: text, json or toon (default: auto)
  -v, --verbose         log every HTTP request to stderr
```

### SEE ALSO

* [store google listing](store_google_listing.md)	 - Show and change the store listing texts

