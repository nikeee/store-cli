package cli

import (
	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/apple"
	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/media"
	"github.com/nikeee/store-cli/internal/metadata"
)

func newApplePreviewsCmd(Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "previews",
		Short: "List, upload and delete app previews",
	}
	makeGroup(cmd)
	cmd.AddCommand(newApplePreviewsListCmd(), newApplePreviewsUploadCmd(), newApplePreviewsDeleteCmd())
	return cmd
}

func newApplePreviewsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [--locale <l>] [--device <d>] [--version <v>]",
		Short: "List the app preview sets of a version",
		Long: "Lists the app preview sets per locale and device with their previews in order. Without --version:\n" +
			"the editable version if there is one, else the live one.\n" +
			"Result: [PreviewSet]",
		Example: "  store apple previews list --locale en-US --device iphone-6.9",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var device *media.Device
			if alias, _ := cmd.Flags().GetString("device"); alias != "" {
				d, err := apple.DeviceForPreviews(alias)
				if err != nil {
					return err
				}
				device = &d
			}
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			v, err := listingVersion(cmd, svc, app)
			if err != nil {
				return err
			}
			locale, _ := cmd.Flags().GetString("locale")
			sets, err := svc.ListPreviews(cmd.Context(), v, locale, device)
			if err != nil {
				return err
			}
			return emit(cmd, sets)
		},
	}
	addMediaListFlags(cmd)
	return cmd
}

func newApplePreviewsUploadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upload --locale <l> --device <d> [--version <v>] [--replace] [--poster-frame <timecode>] <video>...",
		Short: "Upload app previews to the set of one locale and device",
		Long: "Uploads the videos (.mp4, .mov or .m4v) in the given order. Unlike screenshots the device is required,\n" +
			"as there is no size detection for videos; watches have no previews. The preview type is the display\n" +
			"type without APP_ (IPHONE_67 for APP_IPHONE_67). Without --replace the files are\n" +
			"appended to the set (max 3 per set). With --replace: if the set already holds exactly these files in\n" +
			"this order (MD5), nothing happens; otherwise the set is emptied and the files are uploaded in order.\n" +
			"--poster-frame is the timecode of the poster frame (e.g. 00:00:05:00), sent for every file. Each file\n" +
			"is reserved, uploaded, committed with its MD5 and polled until Apple processed it; --timeout is the\n" +
			"time per file. With --dry-run the result is the planned Plan.\n" +
			"Result: PreviewSet",
		Example: "  store apple previews upload --locale en-US --device iphone-6.9 --replace previews/en-US.mp4\n" +
			"  store apple previews upload --locale de-DE --device ipad-13 --poster-frame 00:00:05:00 --timeout 30m tour.mov",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireFlags(cmd, "locale", "device"); err != nil {
				return err
			}
			timeout, _ := cmd.Flags().GetDuration("timeout")
			if timeout <= 0 {
				return clierr.Usagef("invalid --timeout %s (must be positive)", timeout)
			}
			alias, _ := cmd.Flags().GetString("device")
			d, err := apple.DeviceForPreviews(alias)
			if err != nil {
				return err
			}
			poster, _ := cmd.Flags().GetString("poster-frame")
			if err := apple.CheckPosterFrame(poster); err != nil {
				return err
			}
			if err := apple.CheckPreviewFiles(args); err != nil {
				return err
			}
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			v, err := listingVersion(cmd, svc, app)
			if err != nil {
				return err
			}
			locale, _ := cmd.Flags().GetString("locale")
			replace, _ := cmd.Flags().GetBool("replace")
			dry := dryRun(cmd)
			set, change, err := svc.UploadPreviews(cmd.Context(), v, apple.PreviewUpload{
				Locale: locale, Device: d, Paths: args, Replace: replace, PosterFrame: poster, Timeout: timeout, DryRun: dry,
			})
			if err != nil {
				return err
			}
			if dry {
				return emit(cmd, metadata.Plan{Changes: []metadata.Change{change}})
			}
			return emit(cmd, set)
		},
	}
	addMediaSetFlags(cmd, "device of the set, e.g. iphone-6.9 (required)")
	addMediaUploadFlags(cmd)
	cmd.Flags().String("poster-frame", "", "timecode of the poster frame, e.g. 00:00:05:00")
	return cmd
}

func newApplePreviewsDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete --locale <l> --device <d> [--version <v>]",
		Short: "Delete the app preview set of one locale and device",
		Long: "Deletes the whole set. A set that does not exist is a skip (exit 0). With --dry-run the result\n" +
			"is the planned Plan.\n" +
			"Result: PreviewsDeleted",
		Example: "  store apple previews delete --locale en-US --device iphone-6.5",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireFlags(cmd, "locale", "device"); err != nil {
				return err
			}
			alias, _ := cmd.Flags().GetString("device")
			d, err := apple.DeviceForPreviews(alias)
			if err != nil {
				return err
			}
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			v, err := listingVersion(cmd, svc, app)
			if err != nil {
				return err
			}
			locale, _ := cmd.Flags().GetString("locale")
			dry := dryRun(cmd)
			res, change, err := svc.DeletePreviews(cmd.Context(), v, locale, d, dry)
			if err != nil {
				return err
			}
			if dry {
				return emit(cmd, metadata.Plan{Changes: []metadata.Change{change}})
			}
			return emit(cmd, res)
		},
	}
	addMediaSetFlags(cmd, "device of the set, e.g. iphone-6.9 (required)")
	return cmd
}
