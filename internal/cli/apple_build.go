package cli

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/apple"
	"github.com/nikeee/store-cli/internal/clierr"
)

func newAppleBuildCmd(Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "List, inspect and update builds",
	}
	makeGroup(cmd)
	cmd.AddCommand(
		newAppleBuildListCmd(),
		newAppleBuildShowCmd(),
		newAppleBuildWaitCmd(),
		newAppleBuildSetEncryptionCmd(),
		newAppleBuildExpireCmd(),
	)
	return cmd
}

func newAppleBuildListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list",
		Short: "List builds, newest first",
		Long:  "Newest first. Result: [Build].",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			version, _ := cmd.Flags().GetString("version")
			state, _ := cmd.Flags().GetString("state")
			limit, _ := cmd.Flags().GetInt("limit")
			platform, _ := cmd.Flags().GetString("platform")
			if limit < 1 || limit > 200 {
				return clierr.Usagef("invalid --limit %d (use 1 to 200)", limit)
			}
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			builds, err := svc.ListBuilds(cmd.Context(), app, apple.BuildFilter{
				Version: version, State: state, Limit: limit, Platform: platform,
			})
			if err != nil {
				return err
			}
			return emit(cmd, builds)
		},
	}
	cmd.Flags().String("version", "", "only builds of this app version")
	cmd.Flags().String("state", "", "only builds in this processing state: processing, failed, invalid or valid")
	cmd.Flags().Int("limit", 20, "maximum number of builds")
	return cmd
}

func newAppleBuildShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show <build>",
		Short: "Show one build",
		Long:  "Result: Build.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			b, err := svc.ResolveBuild(cmd.Context(), app, args[0], versionFlag(cmd), platformFlag(cmd))
			if err != nil {
				return err
			}
			return emit(cmd, b)
		},
	}
	addBuildVersionFlag(cmd)
	return cmd
}

func newAppleBuildWaitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "wait <build>",
		Short: "Wait until a build exists and is processed",
		Long: "Waits until the build exists and is processed. VALID exits 0; FAILED or INVALID exits 1.\n" +
			"Result: Build.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			timeout, _ := cmd.Flags().GetDuration("timeout")
			if timeout <= 0 {
				return clierr.Usagef("invalid --timeout %s (must be positive)", timeout)
			}
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			b, err := svc.WaitForBuild(cmd.Context(), app, args[0], versionFlag(cmd), platformFlag(cmd), apple.BuildProcessed, timeout)
			if err != nil {
				return err
			}
			return emit(cmd, b)
		},
	}
	addBuildVersionFlag(cmd)
	cmd.Flags().Duration("timeout", 30*time.Minute, "how long to wait, e.g. 90s, 10m, 1h")
	return cmd
}

func newAppleBuildSetEncryptionCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-encryption <build> <true|false>",
		Short: "Set the export compliance answer of a build",
		Long:  "Sets usesNonExemptEncryption (export compliance). Result: Build.",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			var uses bool
			switch args[1] {
			case "true":
				uses = true
			case "false":
			default:
				return clierr.Usagef("invalid value %q (use true or false)", args[1])
			}
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			b, err := svc.ResolveBuild(cmd.Context(), app, args[0], versionFlag(cmd), platformFlag(cmd))
			if err != nil {
				return err
			}
			if dryRun(cmd) {
				b.UsesNonExemptEncryption = &uses
				return emit(cmd, b)
			}
			b, err = svc.SetEncryption(cmd.Context(), b, uses)
			if err != nil {
				return err
			}
			return emit(cmd, b)
		},
	}
	addBuildVersionFlag(cmd)
	addDryRun(cmd)
	return cmd
}

func newAppleBuildExpireCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "expire <build>",
		Short: "Expire a build in TestFlight",
		Long:  "Expires the build in TestFlight. Result: Build.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			b, err := svc.ResolveBuild(cmd.Context(), app, args[0], versionFlag(cmd), platformFlag(cmd))
			if err != nil {
				return err
			}
			if dryRun(cmd) {
				b.Expired = true
				return emit(cmd, b)
			}
			b, err = svc.ExpireBuild(cmd.Context(), b)
			if err != nil {
				return err
			}
			return emit(cmd, b)
		},
	}
	addBuildVersionFlag(cmd)
	addDryRun(cmd)
	return cmd
}

func addBuildVersionFlag(cmd *cobra.Command) {
	cmd.Flags().String("version", "", "app version the build belongs to (needed when the build number exists under several versions)")
}

func versionFlag(cmd *cobra.Command) string {
	v, _ := cmd.Flags().GetString("version")
	return v
}

func platformFlag(cmd *cobra.Command) string {
	v, _ := cmd.Flags().GetString("platform")
	return v
}
