package output

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/nikeee/store-cli/internal/clierr"
)

// Reporter writes progress and warnings to stderr.
type Reporter interface {
	Progressf(format string, args ...any)
	Warnf(format string, args ...any)
}

// TextRenderer is implemented by result types that render their own text form.
type TextRenderer interface {
	RenderText(w io.Writer, s Styles) error
}

// Printer writes results to stdout and errors, progress and warnings to stderr.
type Printer struct {
	mode      Mode
	stdout    io.Writer
	stderr    io.Writer
	styles    Styles // stdout
	errStyles Styles // stderr

	github    bool   // also write GitHub Actions annotations, see AnnotateGitHub
	workspace string // GitHub Actions checkout directory, "" if unknown
}

var _ Reporter = (*Printer)(nil)

// NewPrinter returns a Printer. color only has an effect in text mode; it
// applies to stdout and, unless ColorStderr says otherwise, to stderr.
func NewPrinter(mode Mode, stdout, stderr io.Writer, color bool) *Printer {
	s := NewStyles(color && mode == ModeText)
	return &Printer{mode: mode, stdout: stdout, stderr: stderr, styles: s, errStyles: s}
}

// ColorStderr sets whether stderr gets colors (only in text mode), for when
// stderr is a terminal and stdout is not, or the other way round.
func (p *Printer) ColorStderr(color bool) {
	p.errStyles = NewStyles(color && p.mode == ModeText)
}

// Mode returns the output mode.
func (p *Printer) Mode() Mode { return p.mode }

// Result writes v to stdout in the printer's mode.
func (p *Printer) Result(v any) error {
	switch p.mode {
	case ModeJSON:
		b, err := encodeJSON(v)
		if err != nil {
			return err
		}
		_, err = p.stdout.Write(b)
		return err
	case ModeTOON:
		b, err := encodeTOON(v)
		if err != nil {
			return err
		}
		_, err = p.stdout.Write(b)
		return err
	}
	if r, ok := v.(TextRenderer); ok {
		return r.RenderText(p.stdout, p.styles)
	}
	return renderGeneric(p.stdout, p.styles, v)
}

type errorBody struct {
	Error  string `json:"error"`
	Status int    `json:"status,omitempty"`
	Code   string `json:"code,omitempty"`
	Detail string `json:"detail,omitempty"`
	Hint   string `json:"hint,omitempty"`
}

// Error writes err to stderr: as text, or in json and toon mode as an object.
// With AnnotateGitHub the annotations come first.
func (p *Printer) Error(err error) {
	body := errorBody{Error: err.Error()}
	var ce *clierr.Error
	if errors.As(err, &ce) {
		body.Status, body.Code, body.Detail, body.Hint = ce.Status, ce.Code, ce.Detail, ce.Hint
	}
	if p.github {
		p.annotateError(err, body)
	}
	switch p.mode {
	case ModeJSON, ModeTOON:
		enc := encodeJSON
		if p.mode == ModeTOON {
			enc = encodeTOON
		}
		if b, e := enc(body); e == nil {
			_, _ = p.stderr.Write(b)
			return
		}
	}
	var sb strings.Builder
	sb.WriteString(p.errStyles.Error.Render("error: "+body.Error) + "\n")
	if body.Detail != "" {
		for _, l := range strings.Split(body.Detail, "\n") {
			sb.WriteString("  " + l + "\n")
		}
	}
	if body.Hint != "" {
		sb.WriteString("  hint: " + body.Hint + "\n")
	}
	_, _ = io.WriteString(p.stderr, sb.String())
}

// Progressf writes one progress line to stderr.
func (p *Printer) Progressf(format string, args ...any) {
	_, _ = io.WriteString(p.stderr, p.errStyles.Dim.Render(fmt.Sprintf(format, args...))+"\n")
}

// Warnf writes one "warning: ..." line to stderr.
func (p *Printer) Warnf(format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	_, _ = io.WriteString(p.stderr, p.errStyles.Warn.Render("warning: "+msg)+"\n")
	if p.github {
		p.annotate("warning", clierr.Issue{Message: msg})
	}
}
