package metadata

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/nikeee/store-cli/internal/clierr"
)

// ReadReleaseNotesDir reads <locale>.txt files from dir. A file whose name is
// not a locale, or that is empty, is an error naming the file. A directory
// without any <locale>.txt file is an error too.
func ReadReleaseNotesDir(dir string) (map[string]string, error) {
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil, clierr.New("release notes directory: %v", err)
	}
	out := map[string]string{}
	for _, e := range entries {
		name := e.Name()
		if e.IsDir() || !strings.HasSuffix(name, ".txt") {
			continue
		}
		p := filepath.Join(dir, name)
		locale := strings.TrimSuffix(name, ".txt")
		if !IsLocale(locale) {
			return nil, clierr.New("%s: %q is not a locale name", p, locale)
		}
		b, err := os.ReadFile(p)
		if err != nil {
			return nil, clierr.New("%v", err)
		}
		text := TrimValue(string(b))
		if text == "" {
			return nil, clierr.New("%s: release notes are empty", p)
		}
		out[locale] = text
	}
	if len(out) == 0 {
		return nil, clierr.New("no release notes (<locale>.txt files) in %s", dir)
	}
	return out, nil
}
