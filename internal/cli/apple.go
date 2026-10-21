package cli

import (
	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/clierr"
)

var applePlatforms = []string{"ios", "macos", "tvos", "visionos"}

func newAppleCmd(Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "apple",
		Short: "Manage apps in App Store Connect",
	}
	makeGroup(cmd)
	cmd.PersistentFlags().String("app", "", "bundle ID of the app, e.g. com.example.app (default $APPLE_APPSTORE_BUNDLE_ID)")
	cmd.PersistentFlags().String("platform", "ios", "platform of versions, builds and screenshots: ios, macos, tvos or visionos")
	cmd.PersistentPreRunE = func(c *cobra.Command, _ []string) error {
		p, _ := c.Flags().GetString("platform")
		for _, ok := range applePlatforms {
			if p == ok {
				return nil
			}
		}
		return clierr.Usagef("invalid --platform %q (use ios, macos, tvos or visionos)", p)
	}
	return cmd
}
