package metadata

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode/utf8"

	"github.com/nikeee/store-cli/internal/clierr"
)

// ReadReleaseNotesDir reads <locale>.txt files from dir. A file whose name is
// not a locale, that is empty or has more than maxChars runes is an error
// naming the file; all of them are reported together. A directory without
// any <locale>.txt file is an error too.
func ReadReleaseNotesDir(dir string, maxChars int) (map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, clierr.New("release notes directory: %v", err)
	}
	out := map[string]string{}
	var c clierr.Collector
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".txt") {
			continue
		}
		p := filepath.Join(dir, name)
		locale := strings.TrimSuffix(name, ".txt")
		if !IsLocale(locale) {
			c.Add(clierr.Filef(p, "%q is not a locale name", locale))
			continue
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, clierr.New("%v", err)
		}
		text := TrimValue(string(b))
		if text == "" {
			c.Add(clierr.Filef(p, "release notes are empty"))
			continue
		}
		if n := utf8.RuneCountInString(text); n > maxChars {
			line, col := position(text, maxChars)
			c.Add(clierr.At(p, clierr.Issue{File: p, Line: line, Col: col, Message: fmt.Sprintf("%d characters, limit %d", n, maxChars)}))
			continue
		}
		out[locale] = text
	}
	if err := c.Err(); err != nil {
		return nil, err
	}
	if len(out) == 0 {
		return nil, clierr.New("no release notes (<locale>.txt files) in %s", dir)
	}
	return out, nil
}
