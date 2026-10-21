package cli

import (
	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/metadata"
)

func newAppleReleaseNotesCmd(Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "release-notes",
		Short: "Set the What's New text",
	}
	makeGroup(cmd)
	cmd.AddCommand(newAppleReleaseNotesSetCmd())
	return cmd
}

func newAppleReleaseNotesSetCmd() *cobra.Command {
	var text *TextFlag
	cmd := &cobra.Command{
		Use:   "set <version> (--dir <dir> | --locale <l> --text <text>)",
		Short: "Set What's New of a version",
		Long: "Sets \"What's New\". --dir reads <locale>.txt per locale. A locale without a localization on the\n" +
			"version is an error that suggests `listing locale add`. App Store Connect accepts What's New only\n" +
			"after the first release.\n" +
			"Result: [WhatsNew]",
		Example: "  store apple release-notes set 1.4.0 --locale en-US --text \"Bug fixes\"\n" +
			"  store apple release-notes set 1.4.0 --dir release-notes",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir, _ := cmd.Flags().GetString("dir")
			locale, _ := cmd.Flags().GetString("locale")
			var notes map[string]string
			switch {
			case dir != "" && (locale != "" || text.Changed):
				return clierr.Usagef("--dir cannot be combined with --locale or --text")
			case dir != "":
				var err error
				if notes, err = metadata.ReadReleaseNotesDir(dir); err != nil {
					return err
				}
			case locale != "" && text.Changed:
				notes = map[string]string{locale: text.Value}
			default:
				return clierr.Usagef("pass --dir, or --locale together with --text")
			}
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			v, err := findVersionToChange(cmd, svc, app, args[0])
			if err != nil {
				return err
			}
			set := svc.SetReleaseNotes
			if dryRun(cmd) {
				set = svc.PreviewReleaseNotes
			}
			res, err := set(cmd.Context(), app, v, notes)
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
	text = NewTextFlag("text", func(p string) ([]byte, error) { return cmdDeps(cmd).ReadFile(p) })
	cmd.Flags().String("dir", "", "directory with one <locale>.txt per locale")
	cmd.Flags().String("locale", "", "locale of --text")
	cmd.Flags().Var(text, "text", "the What's New text, or @file to read it from a file (max 4000 characters)")
	addDryRun(cmd)
	return cmd
}
