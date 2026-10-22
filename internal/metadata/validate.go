package metadata

import (
	"fmt"
	"strings"
	"unicode/utf8"

	"github.com/nikeee/store-cli/internal/clierr"
)

// ValidateTexts checks the limits and value rules of the fields present in t.
// All violations are reported together. Limits count runes, not bytes.
func ValidateTexts(fields []Field, t Texts) error {
	return ValidateLocaleTexts("", fields, t)
}

// ValidateLocaleTexts is ValidateTexts with violations prefixed by the
// locale, like "de-DE/subtitle.txt: 31 characters, limit 30".
func ValidateLocaleTexts(locale string, fields []Field, t Texts) error {
	var msgs []string
	for _, f := range fields {
		v, ok := t[f.API]
		if !ok {
			continue
		}
		name := f.File
		if locale != "" {
			name = locale + "/" + f.File
		}
		if n := utf8.RuneCountInString(v); f.MaxChars > 0 && n > f.MaxChars {
			msgs = append(msgs, fmt.Sprintf("%s: %d characters, limit %d", name, n, f.MaxChars))
		}
		// An empty Bool field means "no change": the flag cannot be cleared.
		if f.Bool && v != "" && v != "true" && v != "false" {
			msgs = append(msgs, fmt.Sprintf("%s: must be true or false", name))
		}
	}
	if len(msgs) == 0 {
		return nil
	}
	return clierr.New("%s", strings.Join(msgs, "; "))
}
