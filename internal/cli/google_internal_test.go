package cli

import (
	"testing"

	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/google"
)

func TestCommitOptions(t *testing.T) {
	for _, tt := range []struct {
		args []string
		want google.CommitOptions
	}{
		{[]string{}, google.CommitOptions{}},
		{[]string{"--changes-not-sent-for-review"}, google.CommitOptions{ChangesNotSentForReview: true}},
		{[]string{"--dry-run"}, google.CommitOptions{DryRun: true}},
	} {
		var got google.CommitOptions
		cmd := &cobra.Command{Use: "x", RunE: func(c *cobra.Command, _ []string) error {
			got = commitOptions(c)
			return nil
		}}
		addDryRun(cmd)
		addChangesNotSentForReview(cmd)
		cmd.SetArgs(tt.args)
		if err := cmd.Execute(); err != nil {
			t.Fatal(err)
		}
		if got != tt.want {
			t.Errorf("%v: got %+v", tt.args, got)
		}
	}
}
