package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/config"
	"github.com/nikeee/store-cli/internal/output"
)

type printerKey struct{}

// printerSlot lets Run see the printer that the root PersistentPreRunE
// resolved, so errors are printed in the resolved mode.
type printerSlot struct{ p *output.Printer }

// newPrinter builds the printer for mode: color only in text mode, per stream
// on a terminal with NO_COLOR unset, annotations on GitHub Actions.
func newPrinter(d Deps, mode output.Mode) *output.Printer {
	_, noColor := d.Env(config.EnvNoColor)
	p := output.NewPrinter(mode, d.Stdout, d.Stderr, d.IsTTY && !noColor)
	p.ColorStderr(d.ErrTTY && !noColor)
	if v, _ := d.Env(config.EnvGitHubActions); v == "true" {
		ws, _ := d.Env(config.EnvGitHubWorkspace)
		p.AnnotateGitHub(ws)
	}
	return p
}

// resolvePrinter is the root PersistentPreRunE: it resolves the output mode
// and stores the printer in the command context.
func resolvePrinter(d Deps) func(*cobra.Command, []string) error {
	return func(c *cobra.Command, _ []string) error {
		flag, _ := c.Flags().GetString("output")
		mode, err := output.ResolveMode(flag, d.Env)
		if err != nil {
			return err
		}
		p := newPrinter(d, mode)
		ctx := c.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		if slot, ok := ctx.Value(printerKey{}).(*printerSlot); ok {
			slot.p = p
		}
		c.SetContext(context.WithValue(ctx, printerKey{}, p))
		return nil
	}
}

// printer returns the printer resolved for cmd. Only valid in command bodies.
func printer(cmd *cobra.Command) *output.Printer {
	if ctx := cmd.Context(); ctx != nil {
		if p, ok := ctx.Value(printerKey{}).(*output.Printer); ok {
			return p
		}
	}
	return output.NewPrinter(output.ModeText, cmd.OutOrStdout(), cmd.ErrOrStderr(), false)
}

// emit writes the typed result of a command to stdout.
func emit(cmd *cobra.Command, v any) error {
	return printer(cmd).Result(v)
}
