// Package clierr defines the error type shared by all commands and the
// mapping from errors to process exit codes.
package clierr

import (
	"errors"
	"fmt"
)

// Error is a user-facing failure. Message is one line, starts lower-case and
// has no trailing period.
type Error struct {
	Message string // one line, starts lower-case, no trailing period
	Status  int    // HTTP status, 0 if not from an API
	Code    string // API error code or a store code like "STATE_ERROR"
	Detail  string // API detail text
	Hint    string // next command to run, if known
	Usage   bool   // true: exit code 2
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
