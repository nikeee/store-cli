## store self-upgrade

Replace this binary with the latest release

### Synopsis

Downloads the archive of the latest GitHub release for this platform, verifies it against the
release's checksums.txt and replaces this binary with the store in it. Skips when this binary
already is the latest release or newer; local builds (dev) are always replaced.

Result: SelfUpgrade

```
store self-upgrade [flags]
```

### Examples

```
  store self-upgrade --dry-run
  store self-upgrade
```

### Options

```
      --dry-run   print what would change, change nothing
  -h, --help      help for self-upgrade
```

### Options inherited from parent commands

```
  -o, --output string   output format: text, json or toon (default: auto)
  -v, --verbose         log every HTTP request to stderr
```

### SEE ALSO

* [store](store.md)	 - Manage App Store Connect and Google Play from the command line

