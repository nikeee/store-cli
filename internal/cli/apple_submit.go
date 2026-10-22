package cli

import (
	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/apple"
	"github.com/nikeee/store-cli/internal/metadata"
)

func newAppleSubmitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit <version> [--build N|latest] [--metadata <dir>] [--screenshots <dir>] [--release-notes <dir>] [--release manual|after-approval|scheduled] [--release-date <time>]",
		Short: "Do the whole App Store submission of a version",
		Long: "The whole App Store submission in one command:\n" +
			"1. Find the build (--build, default latest = newest VALID build of <version>).\n" +
			"2. Find or create the version. It must be editable (PREPARE_FOR_SUBMISSION, DEVELOPER_REJECTED,\n" +
			"   REJECTED, METADATA_REJECTED, INVALID_BINARY); otherwise exit 1 with its state.\n" +
			"3. Attach the build. Set the release type only when --release is given (scheduled needs\n" +
			"   --release-date, an RFC 3339 time; --release-date needs --release scheduled).\n" +
			"4. --metadata: run push against this version.\n" +
			"5. --release-notes: set \"What's New\" per locale. While the app has never had a released version,\n" +
			"   this step is skipped with a notice (Apple rejects the field then).\n" +
			"6. Submit as in version submit.\n" +
			"Local inputs are checked before the first API call. --dry-run changes nothing and shows the steps.\n" +
			"Result: SubmitResult",
		Example: "  store apple submit 1.4.0 --metadata fastlane/metadata --release-notes release-notes --release after-approval",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o := apple.SubmitOptions{Version: args[0], Platform: platformFlag(cmd), DryRun: dryRun(cmd)}
			o.Build, _ = cmd.Flags().GetString("build")
			o.MetadataDir, _ = cmd.Flags().GetString("metadata")
			o.ScreenshotsDir, _ = cmd.Flags().GetString("screenshots")
			var err error
			if o.ReleaseType, o.ReleaseDate, err = releaseFlags(cmd); err != nil {
				return err
			}
			if dir, _ := cmd.Flags().GetString("release-notes"); dir != "" {
				notes, err := metadata.ReadReleaseNotesDir(dir)
				if err != nil {
					return err
				}
				o.Notes = notes
			}
			// All local inputs are checked before the first API call, even the app lookup.
			if err := apple.CheckSubmitInputs(o); err != nil {
				return err
			}
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			res, err := svc.Submit(cmd.Context(), app, o)
			if err != nil {
				if len(res.Plan.Changes) > 0 {
					printChanges(cmd, "plan", res.Plan)
				}
				return err
			}
			return emit(cmd, res)
		},
	}
	cmd.Flags().String("build", "latest", "build number to submit, or latest (newest valid build of the version)")
	cmd.Flags().String("metadata", "", "metadata directory to push to the version first")
	addScreenshotsDirFlag(cmd)
	cmd.Flags().String("release-notes", "", "directory with <locale>.txt What's New texts")
	cmd.Flags().String("release", "", "how the version is released: manual, after-approval or scheduled")
	cmd.Flags().String("release-date", "", "release time for --release scheduled, an RFC 3339 time")
	addDryRun(cmd)
	return cmd
}
