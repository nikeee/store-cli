package metadata

import (
	"fmt"
	"path/filepath"
	"unicode/utf8"

	"github.com/nikeee/store-cli/internal/clierr"
)

// ValidateTexts checks the limits and value rules of the fields present in t.
// All violations are reported together. Limits count runes, not bytes.
func ValidateTexts(fields []Field, t Texts) error {
	return ValidateLocaleTexts("", "", fields, t)
}

// ValidateLocaleTexts is ValidateTexts with violations prefixed by the
// locale, like "de-DE/subtitle.txt: 31 characters, limit 30". dir is the
// folder the texts were read from ("" if not from files); each violation is
// then a clierr.Issue at its place in the file.
func ValidateLocaleTexts(locale, dir string, fields []Field, t Texts) error {
	var c clierr.Collector
	for _, f := range fields {
		v, ok := t[f.API]
		if !ok {
			continue
		}
		name := f.File
		if locale != "" {
			name = locale + "/" + f.File
		}
		file := ""
		if dir != "" {
			file = filepath.Join(dir, f.File)
		}
		if n := utf8.RuneCountInString(v); f.MaxChars > 0 && n > f.MaxChars {
			line, col := position(v, f.MaxChars)
			c.Add(clierr.At(name, clierr.Issue{File: file, Line: line, Col: col, Message: fmt.Sprintf("%d characters, limit %d", n, f.MaxChars)}))
		}
		// An empty Bool field means "no change": the flag cannot be cleared.
		if f.Bool && v != "" && v != "true" && v != "false" {
			c.Add(clierr.At(name, clierr.Issue{File: file, Line: 1, Message: "must be true or false"}))
		}
	}
	return c.Err()
}

// position returns the 1-based line and column (in runes) of the rune at
// index i of s. Texts are trimmed only at the end, so it is the place in the
// file too.
func position(s string, i int) (line, col int) {
	line, col = 1, 1
	for _, r := range s {
		if i == 0 {
			break
		}
		i--
		if r == '\n' {
			line, col = line+1, 1
		} else {
			col++
		}
	}
	return line, col
}
