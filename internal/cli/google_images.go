package cli

import (
	"strings"

	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/media"
	"github.com/nikeee/store-cli/internal/metadata"
)

func newGoogleImagesCmd(Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "images",
		Short: "List, upload and delete listing images",
	}
	makeGroup(cmd)
	cmd.AddCommand(newGoogleImagesListCmd(), newGoogleImagesUploadCmd(), newGoogleImagesDeleteCmd())
	return cmd
}

// imageTypeFlag returns the image type named by --type; an unknown one is a
// usage error.
func imageTypeFlag(cmd *cobra.Command) (media.ImageType, error) {
	alias, _ := cmd.Flags().GetString("type")
	t, ok := media.ImageTypeByAlias(alias)
	if !ok {
		return media.ImageType{}, clierr.Usagef("unknown --type %q (use one of: %s)", alias, imageTypeAliases())
	}
	return t, nil
}

func newGoogleImagesListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list --locale <l> [--type <t>]",
		Short: "List the images of a locale",
		Long: "Lists the images of one locale in Play's order with their ID, SHA-256 and preview URL. Without\n" +
			"--type every image type that holds images is listed. --locale takes an App Store style locale,\n" +
			"sent as the Play language code. Types: " + imageTypeAliases() + ".\nResult: [ImageSet]",
		Example: "  store google images list --locale en-US --type phone",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireFlags(cmd, "locale"); err != nil {
				return err
			}
			language, err := playLocaleFlag(cmd)
			if err != nil {
				return err
			}
			var t *media.ImageType
			if cmd.Flags().Changed("type") {
				it, err := imageTypeFlag(cmd)
				if err != nil {
					return err
				}
				t = &it
			}
			svc, err := googleService(cmd)
			if err != nil {
				return err
			}
			res, err := svc.ListImages(cmd.Context(), language, t)
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
	cmd.Flags().String("locale", "", "locale to list, e.g. en-US (required)")
	cmd.Flags().String("type", "", "only this image type")
	return cmd
}

func newGoogleImagesUploadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upload --locale <l> --type <t> [--replace] <image>...",
		Short: "Upload images of one locale and type",
		Long: "Uploads PNG or JPEG files in the given order. Format, alpha channel, pixel size and aspect ratio\n" +
			"and the number of files are checked locally before any API call. Without --replace the files are\n" +
			"appended (at most the maximum of the type per locale). With --replace: if Play already holds exactly\n" +
			"these files in this order (SHA-256), nothing happens; otherwise all images of the type are deleted\n" +
			"and the files uploaded in order. Single-image types (feature-graphic, icon, promo-graphic,\n" +
			"tv-banner) take exactly one file and always replace their image; phone needs 2 to 8 screenshots.\n" +
			"If an upload fails after images were deleted, the whole edit is discarded, so nothing is published;\n" +
			"run the command again. --locale takes an App Store style locale. Types: " + imageTypeAliases() + ".\n" +
			"With --dry-run the result is the planned Plan.\n" +
			"Result: ImageSet",
		Example: "  store google images upload --locale en-US --type phone --replace shots/1.png shots/2.png\n" +
			"  store google images upload --locale en-US --type feature-graphic feature.png",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireFlags(cmd, "locale", "type"); err != nil {
				return err
			}
			language, err := playLocaleFlag(cmd)
			if err != nil {
				return err
			}
			t, err := imageTypeFlag(cmd)
			if err != nil {
				return err
			}
			replace, _ := cmd.Flags().GetBool("replace")
			svc, err := googleService(cmd)
			if err != nil {
				return err
			}
			c := commitOptions(cmd)
			set, change, err := svc.UploadImages(cmd.Context(), language, t, args, replace, c)
			if err != nil {
				return err
			}
			if c.DryRun {
				return emit(cmd, metadata.Plan{Changes: []metadata.Change{change}})
			}
			return emit(cmd, set)
		},
	}
	cmd.Flags().String("locale", "", "locale of the images, e.g. en-US (required)")
	cmd.Flags().String("type", "", "image type (required)")
	cmd.Flags().Bool("replace", false, "make the images of the type exactly these files instead of appending")
	addChangesNotSentForReview(cmd)
	addDryRun(cmd)
	return cmd
}

func newGoogleImagesDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete --locale <l> --type <t>",
		Short: "Delete all images of one locale and type",
		Long: "Deletes all images of the type in the locale. A type without images is a skip (exit 0). --locale\n" +
			"takes an App Store style locale. Types: " + imageTypeAliases() + ".\n" +
			"With --dry-run the result is the planned Plan.\n" +
			"Result: ImagesDeleted",
		Example: "  store google images delete --locale en-US --type tv-banner",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireFlags(cmd, "locale", "type"); err != nil {
				return err
			}
			language, err := playLocaleFlag(cmd)
			if err != nil {
				return err
			}
			t, err := imageTypeFlag(cmd)
			if err != nil {
				return err
			}
			svc, err := googleService(cmd)
			if err != nil {
				return err
			}
			c := commitOptions(cmd)
			res, change, err := svc.DeleteImages(cmd.Context(), language, t, c)
			if err != nil {
				return err
			}
			if c.DryRun {
				return emit(cmd, metadata.Plan{Changes: []metadata.Change{change}})
			}
			return emit(cmd, res)
		},
	}
	cmd.Flags().String("locale", "", "locale of the images, e.g. en-US (required)")
	cmd.Flags().String("type", "", "image type (required)")
	addChangesNotSentForReview(cmd)
	addDryRun(cmd)
	return cmd
}

func imageTypeAliases() string {
	aliases := make([]string, len(media.ImageTypes))
	for i, it := range media.ImageTypes {
		aliases[i] = it.Alias
	}
	return strings.Join(aliases, ", ")
}
