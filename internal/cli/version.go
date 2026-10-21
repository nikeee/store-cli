package cli

import "github.com/spf13/cobra"

// Set at build time via -ldflags -X.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// versionInfo is the result of `store version`.
type versionInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

func newVersionCmd(Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return emit(cmd, versionInfo{Version: version, Commit: commit, Date: date})
		},
	}
}
