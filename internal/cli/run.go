package cli

import (
	"context"
	"io"
	"os"
	"time"

	"github.com/nikeee/store-cli/internal/apple/ascclient"
	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/config"
	"github.com/nikeee/store-cli/internal/google"
	"github.com/nikeee/store-cli/internal/output"
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
	slot := &printerSlot{}
	if err := root.ExecuteContext(context.WithValue(ctx, printerKey{}, slot)); err != nil {
		err = google.FromAPIError(ascclient.MapError(err))
		p := slot.p
		if p == nil {
			// failed before the mode was resolved: honor the environment, else text
			mode, merr := output.ResolveMode("", d.Env)
			if merr != nil {
				mode = output.ModeText
			}
			p = newPrinter(d, mode)
		}
		p.Error(err)
		return clierr.ExitCode(err)
	}
	return 0
}

func isTerminal(f *os.File) bool {
	fi, err := f.Stat()
	return err == nil && fi.Mode()&os.ModeCharDevice != 0
}
