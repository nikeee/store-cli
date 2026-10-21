package cli

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/config"
)

// Deps are the process-level dependencies of the command tree, replaced in tests.
type Deps struct {
	Env      config.Env
	ReadFile func(string) ([]byte, error)
	Stdout   io.Writer
	Stderr   io.Writer
	IsTTY    bool
	Now      func() time.Time
	Sleep    func(ctx context.Context, d time.Duration) error // polling and retries; instant in tests
}

// DefaultDeps returns the production dependencies.
func DefaultDeps() Deps {
	return Deps{
		Env:      os.LookupEnv,
		ReadFile: os.ReadFile,
		Stdout:   os.Stdout,
		Stderr:   os.Stderr,
		IsTTY:    isTerminal(os.Stdout),
		Now:      time.Now,
		Sleep: func(ctx context.Context, d time.Duration) error {
			t := time.NewTimer(d)
			defer t.Stop()
			select {
			case <-ctx.Done():
				return ctx.Err()
			case <-t.C:
				return nil
			}
		},
	}
}

// Run executes the command tree with args and returns the process exit code.
func Run(ctx context.Context, args []string, d Deps) int {
	root := NewRootCmd(d)
	root.SetArgs(args)
	if err := root.ExecuteContext(ctx); err != nil {
		_, _ = fmt.Fprintf(d.Stderr, "error: %s\n", err)
		return clierr.ExitCode(err)
	}
	return 0
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
