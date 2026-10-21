package cli

import (
	"strings"

	"github.com/spf13/pflag"

	"github.com/nikeee/store-cli/internal/clierr"
)

// TextFlag is a pflag.Value for free-text flags. A value starting with "@"
// is read from that file (trailing whitespace trimmed); "@@" escapes a literal
// leading "@".
type TextFlag struct {
	Value   string
	Changed bool // true once the flag was given; named Changed because Set is the pflag method

	name     string
	readFile func(string) ([]byte, error)
}

var _ pflag.Value = (*TextFlag)(nil)

// NewTextFlag returns a TextFlag for the flag called name (used in errors).
func NewTextFlag(name string, readFile func(string) ([]byte, error)) *TextFlag {
	return &TextFlag{name: name, readFile: readFile}
}

func (f *TextFlag) String() string { return f.Value }

func (f *TextFlag) Type() string { return "string" }

func (f *TextFlag) Set(s string) error {
	switch {
	case strings.HasPrefix(s, "@@"):
		s = s[1:]
	case strings.HasPrefix(s, "@"):
		path := s[1:]
		b, err := f.readFile(path)
		if err != nil {
			return clierr.Usagef("--%s: cannot read %s", f.name, path)
		}
		s = strings.TrimRight(string(b), " \t\r\n")
	}
	f.Value, f.Changed = s, true
	return nil
}
