package cli

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/clierr"
)

func TestDryRunFlag(t *testing.T) {
	cmd := &cobra.Command{Use: "x", RunE: func(*cobra.Command, []string) error { return nil }}
	addDryRun(cmd)
	if dryRun(cmd) {
		t.Fatal("dry-run must default to false")
	}
	cmd.SetArgs([]string{"--dry-run"})
	if err := cmd.Execute(); err != nil {
		t.Fatal(err)
	}
	if !dryRun(cmd) {
		t.Fatal("dry-run not set")
	}
}

func TestWrapArgsErrorsCoversLaterCommands(t *testing.T) {
	root := &cobra.Command{Use: "root", SilenceUsage: true, SilenceErrors: true}
	sub := &cobra.Command{Use: "sub", Args: cobra.ExactArgs(1), RunE: func(*cobra.Command, []string) error { return nil }}
	root.AddCommand(sub)
	wrapArgsErrors(root)
	root.SetArgs([]string{"sub"})
	err := root.Execute()
	if err == nil || clierr.ExitCode(err) != 2 {
		t.Fatalf("err=%v exit=%d", err, clierr.ExitCode(err))
	}
}
