package cli

import (
	"testing"

	"github.com/spf13/cobra"
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
