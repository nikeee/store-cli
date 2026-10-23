package cli

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/apple"
	"github.com/nikeee/store-cli/internal/metadata"
	"github.com/nikeee/store-cli/internal/output"
)

func addScreenshotsDirFlag(cmd *cobra.Command) {
	cmd.Flags().String("screenshots", "", "screenshots directory (default: <dir>/../screenshots)")
}

func newAppleInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init <dir> --locale <l> [--locale <l>]... [--screenshots <dir>]",
		Short: "Scaffold the metadata directory for a new app",
		Long: "Scaffolds the metadata directory for a new app: per locale one empty file per row of the field\n" +
			"table, review_information/ with its files, and <screenshots>/<locale>/. Never touches existing\n" +
			"files, so it can be re-run to add a locale. Works offline, needs no credentials. For an app that\n" +
			"already has a listing, use pull instead (an empty file clears its field on push).\n" +
			"Text mode also prints each created file with its character limit.\n" +
			"Result: InitResult",
		Example: "  store apple init fastlane/metadata --locale en-US --locale de-DE",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireFlags(cmd, "locale"); err != nil {
				return err
			}
			locales, _ := cmd.Flags().GetStringArray("locale")
			shots, _ := cmd.Flags().GetString("screenshots")
			res, err := apple.Init(args[0], shots, locales)
			if err != nil {
				return err
			}
			return emit(cmd, initResult{res, scaffoldedField})
		},
	}
	cmd.Flags().StringArray("locale", nil, "locale to scaffold, e.g. en-US (required, repeatable)")
	addScreenshotsDirFlag(cmd)
	return cmd
}

// initResult is an InitResult whose text form names the field and the
// character limit of every created file. fieldOf returns the field of a
// created file; folders have none.
type initResult struct {
	metadata.InitResult
	fieldOf func(path string) (metadata.Field, bool)
}

// RenderText implements output.TextRenderer.
func (r initResult) RenderText(w io.Writer, s output.Styles) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if len(r.Created) == 0 {
		_, _ = fmt.Fprintln(tw, s.Key.Render("created:")+" "+s.Dim.Render("(none)"))
	} else {
		_, _ = fmt.Fprintln(tw, s.Key.Render("created:"))
	}
	for _, p := range r.Created {
		f, ok := r.fieldOf(p)
		switch {
		case !ok:
			_, _ = fmt.Fprintf(tw, "  %s%c\n", p, filepath.Separator)
		case f.MaxChars > 0:
			_, _ = fmt.Fprintf(tw, "  %s\t%s, max %d characters\n", p, f.Help, f.MaxChars)
		case f.MaxBytes > 0:
			_, _ = fmt.Fprintf(tw, "  %s\t%s, max %d bytes\n", p, f.Help, f.MaxBytes)
		default:
			_, _ = fmt.Fprintf(tw, "  %s\t%s\n", p, f.Help)
		}
	}
	if len(r.Existing) > 0 {
		_, _ = fmt.Fprintln(tw, s.Key.Render("existing (left alone):"))
		for _, p := range r.Existing {
			_, _ = fmt.Fprintf(tw, "  %s\n", p)
		}
	}
	return tw.Flush()
}

// scaffoldedField returns the field of a file created by apple init; folders
// have none.
func scaffoldedField(path string) (metadata.Field, bool) {
	fields := metadata.AppleFields
	if filepath.Base(filepath.Dir(path)) == metadata.ReviewDir {
		fields = metadata.AppleReviewFields
	}
	return metadata.FieldByFile(fields, filepath.Base(path))
}

func newApplePushCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push <dir> [--screenshots <dir>] [--version <v>] [--only texts,screenshots,review]",
		Short: "Make App Store Connect match a metadata directory",
		Long: "Makes the store match the directory (fastlane layout):\n" +
			"- Target version: --version, else the one editable version of --platform. None: exit 1.\n" +
			"- A file that exists is the desired value; an empty file clears the field (name.txt must not be\n" +
			"  empty, an empty demo_required.txt changes nothing); a missing file leaves the field alone.\n" +
			"- A locale folder without a store localization creates it. Store locales without a folder are\n" +
			"  left alone.\n" +
			"- Screenshots (default <dir>/../screenshots): per locale and device, the ordered MD5 list is\n" +
			"  compared; any difference replaces the whole set. Devices without local files are left alone.\n" +
			"- review_information/ sets the App Review details.\n" +
			"- What's New (release_notes.txt) is skipped with a warning while the app was never released.\n" +
			"All texts and images are checked before any API call. First the plan is computed and printed\n" +
			"(stderr in text mode), then applied; --dry-run stops after the plan. Unchanged values cause no\n" +
			"API call. When a change fails, push stops with exit 1 and the plan shows which changes were\n" +
			"applied (skip for the others); a screenshot set changed only partly is reported incomplete, run\n" +
			"push again. The action of each change shows what was done.\n" +
			"Result: Plan",
		Example: "  store apple push fastlane/metadata --dry-run\n" +
			"  store apple push fastlane/metadata --only texts,review",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			shots, _ := cmd.Flags().GetString("screenshots")
			only, _ := cmd.Flags().GetStringSlice("only")
			o := apple.PushOptions{Dir: args[0], ScreenshotsDir: shots, Version: versionFlag(cmd), Platform: platformFlag(cmd), Only: only, DryRun: dryRun(cmd)}
			in, err := apple.ReadPushDir(o)
			if err != nil {
				return err
			}
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			p, err := svc.PlanPush(cmd.Context(), app, in)
			if err != nil {
				return err
			}
			text := printer(cmd).Mode() == output.ModeText
			if text {
				printChanges(cmd, "plan", p.Plan)
			}
			if o.DryRun {
				return emit(cmd, p.Plan)
			}
			res, err := svc.ApplyPush(cmd.Context(), p)
			if err != nil {
				if text {
					printChanges(cmd, "result (skip = not applied)", res)
				} else {
					_ = emit(cmd, res)
				}
				return err
			}
			return emit(cmd, res)
		},
	}
	addScreenshotsDirFlag(cmd)
	cmd.Flags().String("version", "", "app version to change (default: the one editable version)")
	cmd.Flags().StringSlice("only", nil, "parts to push, comma separated: "+strings.Join(apple.PushParts, ", ")+" (default: all)")
	addDryRun(cmd)
	return cmd
}

// printChanges writes a plan to stderr: a head line, then one line per change
// with action, scope, locale and field.
func printChanges(cmd *cobra.Command, title string, p metadata.Plan) {
	r := printer(cmd)
	switch n := len(p.Changes); n {
	case 0:
		r.Progressf("%s: no changes", title)
	case 1:
		r.Progressf("%s: 1 change", title)
	default:
		r.Progressf("%s: %d changes", title, n)
	}
	for _, c := range p.Changes {
		parts := []string{string(c.Action), c.Scope}
		if c.Locale != "" {
			parts = append(parts, c.Locale)
		}
		r.Progressf("  %s", strings.Join(append(parts, c.Field), " "))
	}
}

func newApplePullCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "pull <dir> [--version <v>]",
		Short: "Write the store listing into a metadata directory",
		Long: "Writes the text files of every store locale and review_information/ (without the demo password)\n" +
			"into <dir>. A field without a value in the store is written as an empty file, so a value cleared\n" +
			"in the store stays cleared on the next push. Overwrites the files it writes and touches nothing\n" +
			"else. Does not download screenshots (Apple re-encodes them, so they would never match the\n" +
			"originals). Without --version: the editable version if there is one, else the live one.\n" +
			"A store locale that is not a locale name is skipped with a warning.\n" +
			"Result: ApplePullResult",
		Example: "  store apple pull fastlane/metadata",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			v, err := listingVersion(cmd, svc, app)
			if err != nil {
				return err
			}
			res, err := svc.Pull(cmd.Context(), app, v, args[0])
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
	addListingVersionFlag(cmd)
	return cmd
}
