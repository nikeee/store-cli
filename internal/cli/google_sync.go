package cli

import (
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/google"
	"github.com/nikeee/store-cli/internal/metadata"
	"github.com/nikeee/store-cli/internal/output"
)

func newGoogleInitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "init <dir> --locale <l> [--locale <l>]...",
		Short: "Scaffold the Play listing directory for a new app",
		Long: "Scaffolds <dir>/android/<locale>/ per locale: one empty file per row of the field table and the\n" +
			"images/ folders of the image type table. Same rules as store apple init: never touches existing\n" +
			"files, so it can be re-run to add a locale; works offline, needs no credentials. For an app that\n" +
			"already has a listing, use pull instead (an empty file clears its field on push). --locale takes\n" +
			"an App Store style locale or a Play code; locales are written as Play codes.\n" +
			"Result: {created: [path], existing: [path]}. Text mode also prints each created file with its\n" +
			"character limit.",
		Example: "  store google init fastlane/metadata --locale en-US --locale de-DE",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireFlags(cmd, "locale"); err != nil {
				return err
			}
			locales, _ := cmd.Flags().GetStringArray("locale")
			res, err := google.Init(args[0], locales)
			if err != nil {
				return err
			}
			return emit(cmd, initResult{res, googleScaffoldedField})
		},
	}
	cmd.Flags().StringArray("locale", nil, "locale to scaffold, e.g. en-US (required, repeatable)")
	return cmd
}

// googleScaffoldedField returns the listing field of a file created by google
// init; folders have none.
func googleScaffoldedField(path string) (metadata.Field, bool) {
	return metadata.FieldByFile(metadata.GoogleFields, filepath.Base(path))
}

func newGooglePushCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "push <dir> [--only texts,images]",
		Short: "Make the Play listing match a metadata directory",
		Long: "Makes the Play listing match <dir>/android/ (fastlane layout, locale folders named by Play codes):\n" +
			"- A file that exists is the desired value; an empty file clears the field; a missing file leaves\n" +
			"  the field alone.\n" +
			"- A locale folder without a Play listing creates it (its empty files are skipped). Play languages\n" +
			"  without a folder are left alone; no language is deleted.\n" +
			"- Images (<locale>/images/): per type, the ordered SHA-256 list is compared with Play's; any\n" +
			"  difference replaces all images of that type. Types without local files are left alone.\n" +
			"- Release notes are not part of push.\n" +
			"All texts and images are checked before any API call. Everything runs in one edit: first the plan\n" +
			"is computed and printed (stderr in text mode), then the listings and the images are changed and the\n" +
			"edit is committed; --dry-run stops after the plan. With nothing to change nothing is committed.\n" +
			"When anything fails, the edit is deleted, so nothing is published: push exits 1 and the plan shows\n" +
			"which changes had been staged (skip for the others). Result: Plan.",
		Example: "  store google push fastlane/metadata --dry-run\n" +
			"  store google push fastlane/metadata --only texts",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			only, _ := cmd.Flags().GetStringSlice("only")
			in, err := google.ReadPushDir(google.PushOptions{Dir: args[0], Only: only})
			if err != nil {
				return err
			}
			svc, err := googleService(cmd)
			if err != nil {
				return err
			}
			text := printer(cmd).Mode() == output.ModeText
			planned := false
			res, err := svc.Push(cmd.Context(), in, commitOptions(cmd), func(p metadata.Plan) {
				planned = true
				if text {
					printChanges(cmd, "plan", p)
				}
			})
			if err != nil {
				if planned && text {
					printChanges(cmd, "result (skip = not staged, nothing was published)", res)
				} else if planned {
					_ = emit(cmd, res)
				}
				return err
			}
			return emit(cmd, res)
		},
	}
	cmd.Flags().StringSlice("only", nil, "parts to push, comma separated: "+strings.Join(google.PushParts, ", ")+" (default: all)")
	addChangesNotSentForReview(cmd)
	addDryRun(cmd)
	return cmd
}

func newGooglePullCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "pull <dir>",
		Short: "Write the Play listing into a metadata directory",
		Long: "Writes the text files of every Play listing locale into <dir>/android/<locale>/ (Play codes). A\n" +
			"field without a value in Play is written as an empty file, so a value cleared in Play stays cleared\n" +
			"on the next push. Overwrites the files it writes and touches nothing else. No images in v1.\n" +
			"Result: {locales: [string], files: int}.",
		Example: "  store google pull fastlane/metadata",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := googleService(cmd)
			if err != nil {
				return err
			}
			res, err := svc.Pull(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
}
