package metadata

import (
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/nikeee/store-cli/internal/clierr"
)

const (
	reviewDir  = "review_information"
	androidDir = "android"
)

var localeRe = regexp.MustCompile(`^[a-z]{2,3}(-[A-Za-z0-9]{2,8})*$`)

func trimValue(s string) string { return strings.TrimRightFunc(s, unicode.IsSpace) }

// readTexts reads the files of fields from dir. Missing files are absent.
func readTexts(dir string, fields []Field) (Texts, error) {
	t := Texts{}
	for _, f := range fields {
		b, err := os.ReadFile(filepath.Join(dir, f.File))
		if errors.Is(err, fs.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, clierr.New("read %s: %v", filepath.Join(dir, f.File), err)
		}
		t[f.API] = trimValue(string(b))
	}
	return t, nil
}

// readLocales reads every locale folder directly below dir.
func readLocales(dir string, fields []Field) (map[string]Texts, error) {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, fs.ErrNotExist) {
		return map[string]Texts{}, nil
	}
	if err != nil {
		return nil, clierr.New("read %s: %v", dir, err)
	}
	out := map[string]Texts{}
	for _, e := range entries {
		name := e.Name()
		if !e.IsDir() || !localeRe.MatchString(name) {
			continue
		}
		if name == reviewDir || name == androidDir {
			continue
		}
		t, err := readTexts(filepath.Join(dir, name), fields)
		if err != nil {
			return nil, err
		}
		out[name] = t
	}
	return out, nil
}

// ReadAppleTexts reads the App Store locale folders and review_information/
// from a metadata directory. review is empty if the folder does not exist.
func ReadAppleTexts(dir string) (locales map[string]Texts, review Texts, err error) {
	locales, err = readLocales(dir, AppleFields)
	if err != nil {
		return nil, nil, err
	}
	review, err = readTexts(filepath.Join(dir, reviewDir), AppleReviewFields)
	if err != nil {
		return nil, nil, err
	}
	return locales, review, nil
}

// ReadGoogleTexts reads dir/android/<locale>/.
func ReadGoogleTexts(dir string) (map[string]Texts, error) {
	return readLocales(filepath.Join(dir, androidDir), GoogleFields)
}

// WriteTexts writes one file per key present in t, value plus a newline, and
// creates dir. It returns the written paths in field table order.
func WriteTexts(dir string, fields []Field, t Texts) (written []string, err error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, clierr.New("create %s: %v", dir, err)
	}
	for _, f := range fields {
		v, ok := t[f.API]
		if !ok {
			continue
		}
		p := filepath.Join(dir, f.File)
		if err := os.WriteFile(p, []byte(v+"\n"), 0o644); err != nil {
			return written, clierr.New("write %s: %v", p, err)
		}
		written = append(written, p)
	}
	return written, nil
}
