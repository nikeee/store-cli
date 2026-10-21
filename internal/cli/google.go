package cli

import "github.com/spf13/cobra"

func newGoogleCmd(Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "google",
		Short: "Manage apps in Google Play",
	}
	makeGroup(cmd)
	cmd.PersistentFlags().String("app", "", "package name, e.g. com.example.app (default $GOOGLE_PLAY_PACKAGE_NAME)")
	return cmd
}
