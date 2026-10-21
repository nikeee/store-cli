// Package cli builds the cobra command tree.
package cli

import (
	"context"
	"net/http"

	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/httpx"
)

// NewRootCmd builds the full command tree.
func NewRootCmd(d Deps) *cobra.Command {
	root := &cobra.Command{
		Use:           "store",
		Short:         "Manage App Store Connect and Google Play from the command line",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	makeGroup(root)
	// Run the root hook together with the nearest group hook (apple validates --platform).
	cobra.EnableTraverseRunHooks = true
	root.PersistentPreRunE = withDeps(d, resolvePrinter(d))
	root.SetOut(d.Stdout)
	root.SetErr(d.Stderr)
	root.SetFlagErrorFunc(func(_ *cobra.Command, err error) error {
		return clierr.Usagef("%s", err.Error())
	})
	root.PersistentFlags().StringP("output", "o", "", "output format: text, json or toon (default: auto)")
	root.PersistentFlags().BoolP("verbose", "v", false, "log every HTTP request to stderr")

	root.AddCommand(newVersionCmd(d), newAppleCmd(d), newGoogleCmd(d))
	root.InitDefaultCompletionCmd()
	if c, _, err := root.Find([]string{"completion"}); err == nil && c != root {
		makeGroup(c)
	}
	wrapArgsErrors(root)
	return root
}

// wrapArgsErrors makes every positional-argument validation error in the tree
// a usage error (exit code 2), keeping cobra's message. Call it after the
// whole tree is built.
func wrapArgsErrors(cmd *cobra.Command) {
	if orig := cmd.Args; orig != nil {
		cmd.Args = func(c *cobra.Command, args []string) error {
			if err := orig(c, args); err != nil {
				return clierr.Usagef("%s", err)
			}
			return nil
		}
	}
	for _, sub := range cmd.Commands() {
		wrapArgsErrors(sub)
	}
}

// makeGroup turns cmd into a command that only groups subcommands: with no
// arguments it prints help, with an unknown argument it fails with a usage error.
func makeGroup(cmd *cobra.Command) {
	cmd.Args = cobra.ArbitraryArgs
	cmd.RunE = func(c *cobra.Command, args []string) error {
		if len(args) > 0 {
			return clierr.Usagef("unknown command %q for %q", args[0], c.CommandPath())
		}
		return c.Help()
	}
}

// addDryRun registers --dry-run on a mutating command.
func addDryRun(cmd *cobra.Command) {
	cmd.Flags().Bool("dry-run", false, "print what would change, change nothing")
}

func dryRun(cmd *cobra.Command) bool {
	v, _ := cmd.Flags().GetBool("dry-run")
	return v
}

type depsKey struct{}

// withDeps wraps a PersistentPreRunE so that the command context carries d.
func withDeps(d Deps, next func(*cobra.Command, []string) error) func(*cobra.Command, []string) error {
	return func(c *cobra.Command, args []string) error {
		ctx := c.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		c.SetContext(context.WithValue(ctx, depsKey{}, d))
		return next(c, args)
	}
}

// cmdDeps returns the Deps of the command tree. Only valid in command bodies.
func cmdDeps(cmd *cobra.Command) Deps {
	if ctx := cmd.Context(); ctx != nil {
		if d, ok := ctx.Value(depsKey{}).(Deps); ok {
			return d
		}
	}
	return DefaultDeps()
}

// httpClient returns the one HTTP client for API calls of cmd: retries, and a
// redacted request log on stderr with --verbose. Only valid in command bodies.
func httpClient(cmd *cobra.Command) *http.Client {
	verbose, _ := cmd.Flags().GetBool("verbose")
	opts := httpx.Options{Verbose: verbose, Log: cmd.ErrOrStderr()}
	if ctx := cmd.Context(); ctx != nil {
		if d, ok := ctx.Value(depsKey{}).(Deps); ok {
			opts.Log, opts.Sleep = d.Stderr, d.Sleep
		}
	}
	return httpx.NewClient(opts)
}
