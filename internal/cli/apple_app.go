package cli

import "github.com/spf13/cobra"

func newAppleAppCmd(Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "app",
		Short: "Show the apps of the team",
	}
	makeGroup(cmd)
	cmd.AddCommand(
		&cobra.Command{
			Use:     "list",
			Short:   "List all apps of the team",
			Example: "  store apple app list",
			Long:    "All apps of the team. Result: [App].",
			Args:    cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				svc, err := appleServiceNoApp(cmd)
				if err != nil {
					return err
				}
				apps, err := svc.ListApps(cmd.Context())
				if err != nil {
					return err
				}
				return emit(cmd, apps)
			},
		},
		&cobra.Command{
			Use:     "show",
			Short:   "Show the app given by --app",
			Example: "  store apple app show --app com.example.app",
			Long: "The app given by --app. Result: {app: App, versions: [Version], locales: [string]}.\n" +
				"versions holds the newest versions per platform (at most 5).",
			Args: cobra.NoArgs,
			RunE: func(cmd *cobra.Command, _ []string) error {
				svc, app, err := appleService(cmd)
				if err != nil {
					return err
				}
				platform, _ := cmd.Flags().GetString("platform")
				details, err := svc.ShowApp(cmd.Context(), app, platform)
				if err != nil {
					return err
				}
				return emit(cmd, details)
			},
		},
	)
	return cmd
}
