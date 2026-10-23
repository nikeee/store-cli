package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/apple"
	"github.com/nikeee/store-cli/internal/google"
	"github.com/nikeee/store-cli/internal/lint"
)

// lintHelpEnd closes the help of both lint commands.
const lintHelpEnd = "A missing file or image is no finding: push leaves it alone. Exit 1 with errors, with --strict\n" +
	"also with warnings. On GitHub Actions every finding is also an annotation at its file.\n" +
	"Result: LintResult"

func newAppleLintCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lint <dir> [--screenshots <dir>] [--only texts,screenshots,review] [--release-notes <dir>] [--strict]",
		Short: "Check a metadata directory offline",
		Long: "Checks the metadata directory and its screenshots (default <dir>/../screenshots) without\n" +
			"credentials or network and reports every problem at once.\n" +
			"Errors:\n" +
			"- everything push rejects: text limits (keywords in bytes), a name under 2 characters,\n" +
			"  demo_required.txt not true or false, screenshot format, alpha channel, size and count,\n" +
			"  unknown device folders\n" +
			"- an empty description.txt, keywords.txt or support_url.txt (required to submit)\n" +
			"- Apple Watch screenshots of more than one size across the locales\n" +
			"- with --release-notes: the release notes in that directory\n" +
			"Warnings:\n" +
			"- no iphone-6.9 or iphone-6.5 screenshots (--platform ios); iPad screenshots without ipad-13\n" +
			"- keywords shorter than 3 characters, spaces next to commas, repeated keywords, the word\n" +
			"  \"app\", words of the name or subtitle\n" +
			lintHelpEnd,
		Example: "  store apple lint fastlane/metadata\n" +
			"  store apple lint fastlane/metadata --release-notes release-notes --strict",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			shots, _ := cmd.Flags().GetString("screenshots")
			only, _ := cmd.Flags().GetStringSlice("only")
			notes, _ := cmd.Flags().GetString("release-notes")
			res, err := apple.Lint(apple.PushOptions{Dir: args[0], ScreenshotsDir: shots, Platform: platformFlag(cmd), Only: only}, notes)
			if err != nil {
				return err
			}
			return emitLint(cmd, res)
		},
	}
	addScreenshotsDirFlag(cmd)
	cmd.Flags().StringSlice("only", nil, "parts to check, comma separated: "+strings.Join(apple.PushParts, ", ")+" (default: all)")
	addLintFlags(cmd)
	return cmd
}

func newGoogleLintCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "lint <dir> [--only texts,images] [--release-notes <dir>] [--strict]",
		Short: "Check a metadata directory offline",
		Long: "Checks <dir>/android/ without credentials or network and reports every problem at once.\n" +
			"Errors:\n" +
			"- everything push rejects: text limits, folders not named by Play codes, image format, alpha\n" +
			"  channel, size, aspect ratio and count, an icon over 1024 KB\n" +
			"- with --release-notes: the release notes in that directory\n" +
			"Warnings:\n" +
			"- no feature graphic, or no locale with 2 screenshots (required to publish)\n" +
			"- phone sets under 4 screenshots, phone screenshots not 9:16 or 16:9 with 1080 px (needed\n" +
			"  for promotion)\n" +
			"- titles with emoji or emoticons, all capitals, repeated special characters, or ranking and\n" +
			"  promotion terms (#1, best, free, % off)\n" +
			lintHelpEnd,
		Example: "  store google lint fastlane/metadata\n" +
			"  store google lint fastlane/metadata --release-notes release-notes --strict",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			only, _ := cmd.Flags().GetStringSlice("only")
			notes, _ := cmd.Flags().GetString("release-notes")
			res, err := google.Lint(google.PushOptions{Dir: args[0], Only: only}, notes)
			if err != nil {
				return err
			}
			return emitLint(cmd, res)
		},
	}
	cmd.Flags().StringSlice("only", nil, "parts to check, comma separated: "+strings.Join(google.PushParts, ", ")+" (default: all)")
	addLintFlags(cmd)
	return cmd
}

func addLintFlags(cmd *cobra.Command) {
	cmd.Flags().String("release-notes", "", "also check this directory with one <locale>.txt per locale")
	cmd.Flags().Bool("strict", false, "exit 1 on warnings too")
}

// emitLint prints res and returns the error that sets the exit code. On
// GitHub Actions the warnings are annotated here, the errors by that error.
func emitLint(cmd *cobra.Command, res *lint.Result) error {
	p := printer(cmd)
	for _, f := range res.Findings {
		if f.Severity == lint.Warning {
			p.AnnotateWarning(f.Issue())
		}
	}
	if err := emit(cmd, res); err != nil {
		return err
	}
	strict, _ := cmd.Flags().GetBool("strict")
	return res.Err(strict)
}
