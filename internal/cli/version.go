package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// Set at build time via -ldflags -X.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

func newVersionCmd(d Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Args:  cobra.NoArgs,
		RunE: func(*cobra.Command, []string) error {
			_, err := fmt.Fprintf(d.Stdout, "store %s (%s, %s)\n", version, commit, date)
			return err
		},
	}
}
