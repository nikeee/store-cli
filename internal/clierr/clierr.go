// Package clierr defines the error type shared by all commands and the
// mapping from errors to process exit codes.
package clierr

import (
	"errors"
	"fmt"
	"strings"
)

// Error is a user-facing failure. Message is one line, starts lower-case and
// has no trailing period.
type Error struct {
	Message string  // one line, starts lower-case, no trailing period
	Status  int     // HTTP status, 0 if not from an API
	Code    string  // API error code or a store code like "STATE_ERROR"
	Detail  string  // API detail text
	Hint    string  // next command to run, if known
	Usage   bool    // true: exit code 2
	Issues  []Issue // the single problems of Message, if known; CI annotates them
}

// Issue is one problem, in File at Line and Col when known.
type Issue struct {
	File    string // "" if the problem is not in one file; Message is then the whole message
	Line    int    // 1-based; 0 for the whole file
	Col     int    // 1-based, in runes; 0 for the whole line
	Message string // without the file name
}

func (e *Error) Error() string { return e.Message }

// New returns a runtime error (exit code 1).
func New(format string, args ...any) *Error {
	return &Error{Message: fmt.Sprintf(format, args...)}
}

// Usagef returns a usage error (exit code 2).
func Usagef(format string, args ...any) *Error {
	return &Error{Message: fmt.Sprintf(format, args...), Usage: true}
}

// At returns the runtime error "<name>: <is.Message>" with the issue is. An
// issue without file is dropped, the message then stands alone.
func At(name string, is Issue) *Error {
	e := &Error{Message: name + ": " + is.Message}
	if is.File != "" {
		e.Issues = []Issue{is}
	}
	return e
}

// Filef returns the runtime error "<path>: <message>" for a problem with the
// whole file path.
func Filef(path, format string, args ...any) *Error {
	return At(path, Issue{File: path, Message: fmt.Sprintf(format, args...)})
}

// Collector gathers problems to report them together as one error.
type Collector struct {
	msgs   []string
	issues []Issue
}

// Add adds the problem err; nil is ignored. Its issues are kept, an error
// without issues is one issue without file.
func (c *Collector) Add(err error) {
	if err == nil {
		return
	}
	c.msgs = append(c.msgs, err.Error())
	var e *Error
	if errors.As(err, &e) && len(e.Issues) > 0 {
		c.issues = append(c.issues, e.Issues...)
		return
	}
	c.issues = append(c.issues, Issue{Message: err.Error()})
}

// Err returns the problems as one runtime error, messages joined by "; ", or
// nil if there are none.
func (c *Collector) Err() error {
	if len(c.msgs) == 0 {
		return nil
	}
	return &Error{Message: strings.Join(c.msgs, "; "), Issues: c.issues}
}

// ExitCode maps an error to the process exit code: nil is 0, usage errors
// (including wrapped ones) are 2, everything else is 1.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var e *Error
	if errors.As(err, &e) && e.Usage {
		return 2
	}
	return 1
}
