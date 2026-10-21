package cli

import "github.com/spf13/cobra"

// Set at build time via -ldflags -X.
var (
	version = "dev"
	commit  = "none"
	date    = "unknown"
)

// VersionInfo is the result of `store version`.
type VersionInfo struct {
	Version string `json:"version"`
	Commit  string `json:"commit"`
	Date    string `json:"date"`
}

func newVersionCmd(Deps) *cobra.Command {
	return &cobra.Command{
		Use:   "version",
		Short: "Print the version",
		Long: "Prints the version, commit and build date of this binary as {version, commit, date}.\n" +
			"Local builds report dev.\n" +
			"Result: VersionInfo",
		Example: "  store version\n  store version -o json",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return emit(cmd, VersionInfo{Version: version, Commit: commit, Date: date})
		},
	}
}
