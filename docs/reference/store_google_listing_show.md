## store google listing show

Show the listing texts per locale

### Synopsis

Shows the title, short and full description and promo video per listing language. --locale takes an
App Store style locale, sent as the Play language code.

Result: [Listing]

```
store google listing show [flags]
```

### Examples

```
  store google listing show --locale de-DE
```

### Options

```
  -h, --help            help for show
      --locale string   only this locale
```

### Options inherited from parent commands

```
      --app string      package name, e.g. com.example.app (default $GOOGLE_PLAY_PACKAGE_NAME)
  -o, --output string   output format: text, json or toon (default: auto)
  -v, --verbose         log every HTTP request to stderr
```

### SEE ALSO

* [store google listing](store_google_listing.md)	 - Show and change the store listing texts

