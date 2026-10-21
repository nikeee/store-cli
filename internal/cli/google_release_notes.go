package cli

import (
	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/google"
	"github.com/nikeee/store-cli/internal/metadata"
)

func newGoogleReleaseNotesCmd(Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release-notes",
		Short: "Set the release notes of a release",
	}
	makeGroup(cmd)
	cmd.AddCommand(newGoogleReleaseNotesSetCmd())
	return cmd
}

func newGoogleReleaseNotesSetCmd() *cobra.Command {
	var text *TextFlag
	cmd := &cobra.Command{
		Use:   "set <track> (--dir <dir> | --locale <l> --text <text>)",
		Short: "Set the release notes of a release on a track",
		Long: "Sets the release notes of a release on a track, by --release-name or, by default, the newest\n" +
			"release by version code. --dir reads <locale>.txt per locale. The notes of other languages stay.\n" +
			"Max 500 characters per locale; locales the listing does not have are dropped with a warning.\n" +
			"The other releases of the track are not changed.\n" +
			"Result: Release",
		Example: "  store google release-notes set production --locale en-US --text \"Bug fixes\"\n" +
			"  store google release-notes set internal --release-name v1.4.0 --dir release-notes",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			f := cmd.Flags()
			dir, _ := f.GetString("dir")
			loc, _ := f.GetString("locale")
			var notes map[string]string
			switch {
			case dir != "" && (loc != "" || text.Changed):
				return clierr.Usagef("--dir cannot be combined with --locale or --text")
			case dir != "":
				var err error
				if notes, err = metadata.ReadReleaseNotesDir(dir); err != nil {
					return err
				}
			case loc != "" && text.Changed:
				if !metadata.IsLocale(loc) {
					return clierr.Usagef("invalid --locale %q", loc)
				}
				t := metadata.TrimValue(text.Value)
				if t == "" {
					return clierr.Usagef("--text is empty")
				}
				notes = map[string]string{loc: t}
			default:
				return clierr.Usagef("pass --dir, or --locale together with --text")
			}
			if err := google.ValidateReleaseNotes(notes); err != nil {
				return err
			}
			svc, err := googleService(cmd)
			if err != nil {
				return err
			}
			name, _ := f.GetString("release-name")
			res, err := svc.SetReleaseNotes(cmd.Context(), args[0], name, notes, commitOptions(cmd))
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
	text = NewTextFlag("text", func(p string) ([]byte, error) { return cmdDeps(cmd).ReadFile(p) })
	f := cmd.Flags()
	f.String("release-name", "", "release to change (default: the newest by version code)")
	f.String("dir", "", "directory with one <locale>.txt per locale")
	f.String("locale", "", "locale of --text")
	f.Var(text, "text", "the release notes, or @file to read them from a file (max 500 characters)")
	addChangesNotSentForReview(cmd)
	addDryRun(cmd)
	return cmd
}
