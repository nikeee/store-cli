package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/config"
	"github.com/nikeee/store-cli/internal/google"
)

func newGoogleCmd(d Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "google",
		Short: "Manage apps in Google Play",
	}
	makeGroup(cmd)
	cmd.PersistentFlags().String("app", "", "package name, e.g. com.example.app (default $GOOGLE_PLAY_PACKAGE_NAME)")
	cmd.AddCommand(newGoogleAuthCmd(d), newGoogleBundleCmd(d), newGoogleTrackCmd(d), newGoogleReleaseNotesCmd(d), newGoogleDetailsCmd(d))
	return cmd
}

// googleService returns the Google Play service for the app given by --app
// or GOOGLE_PLAY_PACKAGE_NAME. Only valid in command bodies.
func googleService(cmd *cobra.Command) (*google.Service, error) {
	d := cmdDeps(cmd)
	pkg, _ := cmd.Flags().GetString("app")
	if pkg == "" {
		v, _ := d.Env(config.EnvGooglePackageName)
		pkg = strings.TrimSpace(v)
	}
	if pkg == "" {
		return nil, clierr.Usagef("no app given: pass --app <package name> or set %s", config.EnvGooglePackageName)
	}
	key, err := config.LoadGoogleServiceAccount(d.Env, d.ReadFile)
	if err != nil {
		return nil, err
	}
	baseURL, _ := d.Env(config.EnvInternalGoogleAPIURL)
	return google.NewService(cmd.Context(), key, httpClient(cmd), baseURL, pkg, printer(cmd))
}

// addChangesNotSentForReview registers --changes-not-sent-for-review on a
// mutating Google command.
func addChangesNotSentForReview(cmd *cobra.Command) {
	cmd.Flags().Bool("changes-not-sent-for-review", false,
		"commit without sending the changes for review; needed when Play Console requires sending them manually")
}

// commitOptions returns how the command's edit ends, from
// --changes-not-sent-for-review and --dry-run.
func commitOptions(cmd *cobra.Command) google.CommitOptions {
	notSent, _ := cmd.Flags().GetBool("changes-not-sent-for-review")
	return google.CommitOptions{ChangesNotSentForReview: notSent, DryRun: dryRun(cmd)}
}
