## store google auth check

Verify the credentials by creating and deleting an edit

### Synopsis

Creates and deletes an edit for --app.

Result: {clientEmail, app, ok}.

```
store google auth check [flags]
```

### Examples

```
  store google auth check --app com.example.app
```

### Options

```
  -h, --help   help for check
```

### Options inherited from parent commands

```
      --app string      package name, e.g. com.example.app (default $GOOGLE_PLAY_PACKAGE_NAME)
  -o, --output string   output format: text, json or toon (default: auto)
  -v, --verbose         log every HTTP request to stderr
```

### SEE ALSO

* [store google auth](store_google_auth.md)	 - Check the Google Play credentials

