package metadata

import (
	"fmt"
	"path/filepath"
	"unicode/utf8"

	"github.com/nikeee/store-cli/internal/clierr"
)

// ValidateTexts checks the limits and value rules of the fields present in t.
// All violations are reported together. MaxChars and MinChars count runes,
// MaxBytes counts bytes.
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
		n := utf8.RuneCountInString(v)
		if f.MaxChars > 0 && n > f.MaxChars {
			line, col := Position(v, f.MaxChars)
			c.Add(clierr.At(name, clierr.Issue{File: file, Line: line, Col: col, Message: fmt.Sprintf("%d characters, limit %d", n, f.MaxChars)}))
		}
		if f.MaxBytes > 0 && len(v) > f.MaxBytes {
			line, col := Position(v, utf8.RuneCountInString(v[:lastRuneStart(v, f.MaxBytes)]))
			c.Add(clierr.At(name, clierr.Issue{File: file, Line: line, Col: col, Message: fmt.Sprintf("%d bytes, limit %d", len(v), f.MaxBytes)}))
		}
		if f.MinChars > 0 && v != "" && n < f.MinChars {
			unit := "characters"
			if n == 1 {
				unit = "character"
			}
			c.Add(clierr.At(name, clierr.Issue{File: file, Line: 1, Message: fmt.Sprintf("%d %s, at least %d", n, unit, f.MinChars)}))
		}
		// An empty Bool field means "no change": the flag cannot be cleared.
		if f.Bool && v != "" && v != "true" && v != "false" {
			c.Add(clierr.At(name, clierr.Issue{File: file, Line: 1, Message: "must be true or false"}))
		}
	}
	return c.Err()
}

// lastRuneStart returns the byte index of the rune of s that ends past limit
// bytes.
func lastRuneStart(s string, limit int) int {
	for i, r := range s {
		if i+utf8.RuneLen(r) > limit {
			return i
		}
	}
	return len(s)
}

// Position returns the 1-based line and column (in runes) of the rune at
// index i of s. Texts are trimmed only at the end, so it is the place in the
// file too.
func Position(s string, i int) (line, col int) {
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
