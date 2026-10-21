package cli

import (
	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/apple"
	"github.com/nikeee/store-cli/internal/clierr"
)

func newAppleTestflightCmd(Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "testflight",
		Short: "Manage TestFlight",
	}
	makeGroup(cmd)
	cmd.AddCommand(newAppleTestflightNotesCmd(), newAppleTestflightGroupCmd(), newAppleTestflightTesterCmd(), newAppleTestflightSubmitCmd())
	return cmd
}

func newAppleTestflightNotesCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "notes",
		Short: "Manage the What to Test notes of builds",
	}
	makeGroup(cmd)
	cmd.AddCommand(newAppleTestflightNotesSetCmd())
	return cmd
}

func newAppleTestflightNotesSetCmd() *cobra.Command {
	var text *TextFlag
	cmd := &cobra.Command{
		Use:   "set <build>",
		Short: "Set the What to Test text of a build",
		Long: "Sets the \"What to Test\" text (max 4000 characters). Works while the build is still processing.\n" +
			"A text starting with @ is read from that file. Result: {build, locale, text}.",
		Example: "  store apple testflight notes set 45 --text \"Fixed login\"\n" +
			"  store apple testflight notes set latest --text @notes.txt --locale de-DE",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !text.Changed {
				return clierr.Usagef("--text is required")
			}
			locale, _ := cmd.Flags().GetString("locale")
			if err := apple.CheckBuildNotes(text.Value); err != nil {
				return err
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
				n, err := svc.PreviewBuildNotes(cmd.Context(), b, locale, text.Value)
				if err != nil {
					return err
				}
				return emit(cmd, n)
			}
			n, err := svc.SetBuildNotes(cmd.Context(), b, locale, text.Value)
			if err != nil {
				return err
			}
			return emit(cmd, n)
		},
	}
	text = NewTextFlag("text", func(p string) ([]byte, error) { return cmdDeps(cmd).ReadFile(p) })
	cmd.Flags().Var(text, "text", "the What to Test text, or @file to read it from a file (max 4000 characters)")
	cmd.Flags().String("locale", "en-US", "locale of the text")
	addBuildVersionFlag(cmd)
	addDryRun(cmd)
	return cmd
}
