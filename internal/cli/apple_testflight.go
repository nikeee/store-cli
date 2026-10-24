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
			"A text starting with @ is read from that file. A build appears in App Store Connect only some\n" +
			"minutes after its upload: --wait-for-build waits up to that long for it instead of failing.\n" +
			"Result: BuildNotes",
		Example: "  store apple testflight notes set 45 --text \"Fixed login\"\n" +
			"  store apple testflight notes set latest --text @notes.txt --locale de-DE\n" +
			"  store apple testflight notes set 46 --text @notes.txt --wait-for-build 15m",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if !text.Changed {
				return clierr.Usagef("--text is required")
			}
			locale, _ := cmd.Flags().GetString("locale")
			wait, _ := cmd.Flags().GetDuration("wait-for-build")
			if wait < 0 {
				return clierr.Usagef("invalid --wait-for-build %s (must be positive)", wait)
			}
			if wait > 0 && args[0] == "latest" {
				return clierr.Usagef("--wait-for-build needs a build number, not latest")
			}
			if err := apple.CheckBuildNotes(text.Value); err != nil {
				return err
			}
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			var b apple.Build
			if wait > 0 {
				b, err = svc.WaitForBuild(cmd.Context(), app, args[0], versionFlag(cmd), platformFlag(cmd), apple.BuildExists, wait)
			} else {
				b, err = svc.ResolveBuild(cmd.Context(), app, args[0], versionFlag(cmd), platformFlag(cmd))
			}
			if err != nil {
				return err
			}
			n, err := svc.SetBuildNotes(cmd.Context(), b, locale, text.Value, dryRun(cmd))
			if err != nil {
				return err
			}
			return emit(cmd, n)
		},
	}
	text = NewTextFlag("text", func(p string) ([]byte, error) { return cmdDeps(cmd).ReadFile(p) })
	cmd.Flags().Var(text, "text", "the What to Test text, or @file to read it from a file (max 4000 characters)")
	cmd.Flags().String("locale", "en-US", "locale of the text")
	cmd.Flags().Duration("wait-for-build", 0, "wait up to this long for the build to appear, e.g. 15m")
	addBuildVersionFlag(cmd)
	addDryRun(cmd)
	return cmd
}
