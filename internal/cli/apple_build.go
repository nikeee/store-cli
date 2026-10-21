package cli

import (
	"strings"
	"time"

	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/apple"
	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/config"
)

func newAppleBuildCmd(Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "build",
		Short: "List, inspect and update builds",
	}
	makeGroup(cmd)
	cmd.AddCommand(
		newAppleBuildUploadCmd(),
		newAppleBuildListCmd(),
		newAppleBuildShowCmd(),
		newAppleBuildWaitCmd(),
		newAppleBuildSetEncryptionCmd(),
		newAppleBuildExpireCmd(),
	)
	return cmd
}

func newAppleBuildUploadCmd() *cobra.Command {
	var notes *TextFlag
	cmd := &cobra.Command{
		Use:   "upload <ipa>",
		Short: "Upload an IPA to App Store Connect",
		Long: "Uploads an IPA through the App Store Connect API. Needs no Xcode, altool or Transporter and runs\n" +
			"on Linux. Reads bundle ID, version and build number from the IPA; --app is optional and must\n" +
			"match the IPA when given. Creates a build upload, uploads the file in the parts Apple asks for,\n" +
			"commits it with its MD5 checksum and waits until Apple accepted the upload. FAILED prints\n" +
			"Apple's errors and exits 1; Apple's warnings go to stderr.\n" +
			"--notes sets the TestFlight \"What to Test\" text and --uses-non-exempt-encryption the export\n" +
			"compliance answer as soon as the build exists. --wait additionally waits until the build is\n" +
			"processed (VALID). --timeout bounds the whole command.\n" +
			"Result: {app, appId, version, build, buildUploadId, buildId, state}. buildId is empty when\n" +
			"neither --wait, --notes nor --uses-non-exempt-encryption made the command wait for the build.",
		Example: "  store apple build upload app.ipa --notes @whats-new.txt --uses-non-exempt-encryption false",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			timeout, _ := cmd.Flags().GetDuration("timeout")
			if timeout <= 0 {
				return clierr.Usagef("invalid --timeout %s (must be positive)", timeout)
			}
			locale, _ := cmd.Flags().GetString("notes-locale")
			wait, _ := cmd.Flags().GetBool("wait")
			o := apple.UploadOptions{
				Path: args[0], Platform: platformFlag(cmd), Wait: wait, Timeout: timeout,
				Notes: notes.Value, NotesLocale: locale,
			}
			if v, _ := cmd.Flags().GetString("uses-non-exempt-encryption"); v != "" {
				switch v {
				case "true", "false":
					uses := v == "true"
					o.Encryption = &uses
				default:
					return clierr.Usagef("invalid --uses-non-exempt-encryption %q (use true or false)", v)
				}
			}
			o.App, _ = cmd.Flags().GetString("app")
			if o.App == "" {
				v, _ := cmdDeps(cmd).Env(config.EnvAppleBundleID)
				o.App = strings.TrimSpace(v)
			}
			svc, err := appleServiceNoApp(cmd)
			if err != nil {
				return err
			}
			var res apple.UploadResult
			if dryRun(cmd) {
				res, err = svc.PreviewUpload(cmd.Context(), o)
			} else {
				res, err = svc.UploadBuild(cmd.Context(), o)
			}
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
	notes = NewTextFlag("notes", func(p string) ([]byte, error) { return cmdDeps(cmd).ReadFile(p) })
	cmd.Flags().Var(notes, "notes", "TestFlight What to Test text, or @file to read it from a file (max 4000 characters)")
	cmd.Flags().String("notes-locale", "en-US", "locale of --notes")
	cmd.Flags().String("uses-non-exempt-encryption", "", "export compliance answer: true or false")
	cmd.Flags().Bool("wait", false, "also wait until the build is processed (VALID)")
	cmd.Flags().Duration("timeout", 30*time.Minute, "how long the whole command may take, e.g. 90s, 10m, 1h")
	addDryRun(cmd)
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
