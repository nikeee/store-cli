package cli

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/google"
)

func newGoogleBundleCmd(Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "bundle",
		Short: "Upload app bundles",
	}
	makeGroup(cmd)
	cmd.AddCommand(newGoogleBundleUploadCmd())
	return cmd
}

// bundleStatuses maps --status to the API spelling.
var bundleStatuses = map[string]string{
	"completed":   google.StatusCompleted,
	"draft":       google.StatusDraft,
	"in-progress": google.StatusInProgress,
	"halted":      google.StatusHalted,
}

func newGoogleBundleUploadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upload <aab>",
		Short: "Upload an AAB and release it on tracks",
		Long: "Uploads the AAB (resumable upload, streamed from disk), optionally the deobfuscation file for\n" +
			"its version code, and creates one release per --track with that version code, all in one edit.\n" +
			"--status defaults to completed; --rollout implies in-progress; in-progress and halted need --rollout. Apps that were never published\n" +
			"only accept --status draft; the error says so. Release notes max 500 characters per locale;\n" +
			"locales the listing does not have are dropped with a warning.\n" +
			"A completed release replaces the track's releases. An in-progress or halted release keeps the\n" +
			"completed one beside it. A draft keeps the releases that are not drafts.\n" +
			"With --dry-run the edit is created and deleted; nothing is uploaded or changed. The result then\n" +
			"has versionCode 0 and releases without versionCodes.\n" +
			"Result: BundleUploadResult",
		Example: "  store google bundle upload app.aab --track internal --release-name v1.4.0 --release-notes release-notes",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireFlags(cmd, "track"); err != nil {
				return err
			}
			o, err := bundleOptions(cmd, args[0])
			if err != nil {
				return err
			}
			svc, err := googleService(cmd)
			if err != nil {
				return err
			}
			res, err := svc.UploadBundle(cmd.Context(), o)
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
	f := cmd.Flags()
	f.StringArray("track", nil, "track to release on: internal, alpha, beta, production or a custom track (repeatable)")
	f.String("release-name", "", "release name")
	f.String("status", "completed", "release status: completed, draft, in-progress or halted")
	f.Float64("rollout", 0, "staged rollout fraction, greater than 0 and less than 1; implies --status in-progress")
	f.String("release-notes", "", "directory with one <locale>.txt per locale")
	f.String("mapping", "", "deobfuscation file (mapping.txt) for the bundle")
	addChangesNotSentForReview(cmd)
	addDryRun(cmd)
	return cmd
}

// bundleOptions validates every local input and builds the options. It
// sends no request.
func bundleOptions(cmd *cobra.Command, aab string) (google.BundleUploadOptions, error) {
	f := cmd.Flags()
	o := google.BundleUploadOptions{Path: aab, Commit: commitOptions(cmd)}
	o.Tracks = stringArray(f, "track")
	o.ReleaseName, _ = f.GetString("release-name")
	var err error
	if o.Status, o.UserFraction, err = releaseStatus(cmd); err != nil {
		return o, err
	}
	if err := checkFile(aab); err != nil {
		return o, err
	}
	if o.Mapping, _ = f.GetString("mapping"); o.Mapping != "" {
		if err := checkFile(o.Mapping); err != nil {
			return o, err
		}
	}
	if dir, _ := f.GetString("release-notes"); dir != "" {
		notes, err := google.ReadReleaseNotesDir(dir)
		if err != nil {
			return o, err
		}
		if err := google.ValidateReleaseNotes(notes); err != nil {
			return o, err
		}
		o.Notes = notes
	}
	return o, nil
}

// releaseStatus reads --status and --rollout of a command that creates a
// release: the status in the API spelling and the user fraction. --rollout
// implies in-progress; in-progress and halted need --rollout. It sends no
// request.
func releaseStatus(cmd *cobra.Command) (string, float64, error) {
	f := cmd.Flags()
	name, _ := f.GetString("status")
	status, ok := bundleStatuses[name]
	if !ok {
		return "", 0, clierr.Usagef("invalid --status %q (use completed, draft, in-progress or halted)", name)
	}
	staged := status == google.StatusInProgress || status == google.StatusHalted
	if !f.Changed("rollout") {
		if staged {
			return "", 0, clierr.Usagef("--status %s needs --rollout <fraction>", name)
		}
		return status, 0, nil
	}
	rollout, _ := f.GetFloat64("rollout")
	if !(rollout > 0 && rollout < 1) {
		return "", 0, clierr.Usagef("invalid --rollout %v (must be greater than 0 and less than 1)", rollout)
	}
	switch {
	case !f.Changed("status"):
		status = google.StatusInProgress
	case !staged:
		return "", 0, clierr.Usagef("--rollout needs --status in-progress or halted, not %s", name)
	}
	return status, rollout, nil
}

// checkFile reports a missing, unreadable or non-regular file.
func checkFile(path string) error {
	st, err := os.Stat(path)
	if err != nil {
		return clierr.New("%v", err)
	}
	if !st.Mode().IsRegular() {
		return clierr.New("%s is not a file", path)
	}
	fh, err := os.Open(path)
	if err != nil {
		return clierr.New("%v", err)
	}
	_ = fh.Close()
	return nil
}
