## store version

Print the version

### Synopsis

Prints the version, commit and build date of this binary as {version, commit, date}.
Local builds report dev.

Result: VersionInfo

```
store version [flags]
```

### Examples

```
  store version
  store version -o json
```

### Options

```
  -h, --help   help for version
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json or toon (default: auto)
  -v, --verbose         log every HTTP request to stderr
```

### SEE ALSO

* [store](store.md)	 - Manage App Store Connect and Google Play from the command line

