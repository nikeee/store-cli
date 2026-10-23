// Package lint holds the findings of the lint commands: their order, text
// form and the error that sets the exit code.
package lint

import (
	"cmp"
	"errors"
	"fmt"
	"io"
	"slices"
	"strconv"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/output"
)

// Severity says whether a finding fails the lint.
type Severity string

const (
	Error   Severity = "error"   // the store rejects it, or it blocks submitting or publishing
	Warning Severity = "warning" // accepted, but against a store recommendation
)

// Finding is one problem, in File at Line and Col when known (see
// clierr.Issue).
type Finding struct {
	Severity Severity `json:"severity"`
	File     string   `json:"file,omitempty"`
	Line     int      `json:"line,omitempty"`
	Col      int      `json:"col,omitempty"`
	Message  string   `json:"message"`
}

// Issue returns f without its severity.
func (f Finding) Issue() clierr.Issue {
	return clierr.Issue{File: f.File, Line: f.Line, Col: f.Col, Message: f.Message}
}

// Result is the result of a lint command.
type Result struct {
	Findings []Finding `json:"findings"`
	Errors   int       `json:"errors"`
	Warnings int       `json:"warnings"`
}

// Add adds the problem is with severity s.
func (r *Result) Add(s Severity, is clierr.Issue) {
	r.Findings = append(r.Findings, Finding{Severity: s, File: is.File, Line: is.Line, Col: is.Col, Message: is.Message})
	if s == Error {
		r.Errors++
	} else {
		r.Warnings++
	}
}

// AddError adds every issue of err as an error; an error without issues is
// one finding without file. nil is ignored.
func (r *Result) AddError(err error) {
	if err == nil {
		return
	}
	var e *clierr.Error
	if !errors.As(err, &e) || len(e.Issues) == 0 {
		r.Add(Error, clierr.Issue{Message: err.Error()})
		return
	}
	for _, is := range e.Issues {
		r.Add(Error, is)
	}
}

// Sort orders the findings: those without file first, in the order they were
// added, then by file, line and column.
func (r *Result) Sort() {
	// "" sorts first, and stable keeps the order of the findings without file
	slices.SortStableFunc(r.Findings, func(a, b Finding) int {
		return cmp.Or(cmp.Compare(a.File, b.File), cmp.Compare(a.Line, b.Line), cmp.Compare(a.Col, b.Col))
	})
}

// Err returns the error that makes the command exit 1: with errors, or with
// warnings if strict. Its issues are the errors, so CI annotates each once.
func (r Result) Err(strict bool) error {
	if r.Errors == 0 && (!strict || r.Warnings == 0) {
		return nil
	}
	e := clierr.New("%s", r.summary())
	if r.Errors == 0 {
		e.Message += " (--strict)"
	}
	for _, f := range r.Findings {
		if f.Severity == Error {
			e.Issues = append(e.Issues, f.Issue())
		}
	}
	return e
}

func (r Result) summary() string {
	if r.Errors == 0 && r.Warnings == 0 {
		return "no problems"
	}
	return plural(r.Errors, "error") + ", " + plural(r.Warnings, "warning")
}

func plural(n int, word string) string {
	if n == 1 {
		return "1 " + word
	}
	return fmt.Sprintf("%d %ss", n, word)
}

// RenderText implements output.TextRenderer: one "file:line:col: severity:
// message" line per finding, then the counts.
func (r Result) RenderText(w io.Writer, s output.Styles) error {
	for _, f := range r.Findings {
		sev := s.Error.Render(string(f.Severity))
		if f.Severity == Warning {
			sev = s.Warn.Render(string(f.Severity))
		}
		if _, err := fmt.Fprintf(w, "%s%s: %s\n", location(f), sev, f.Message); err != nil {
			return err
		}
	}
	_, err := fmt.Fprintln(w, r.summary())
	return err
}

// location is "file:line:col: " with the parts known, "" without file.
func location(f Finding) string {
	if f.File == "" {
		return ""
	}
	loc := f.File
	if f.Line > 0 {
		loc += ":" + strconv.Itoa(f.Line)
		if f.Col > 0 {
			loc += ":" + strconv.Itoa(f.Col)
		}
	}
	return loc + ": "
}
