package cli

import (
	"time"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"

	"github.com/nikeee/store-cli/internal/apple"
	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/media"
	"github.com/nikeee/store-cli/internal/metadata"
)

func newAppleScreenshotsCmd(Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "screenshots",
		Short: "List, upload and delete screenshots",
	}
	makeGroup(cmd)
	cmd.AddCommand(newAppleScreenshotsListCmd(), newAppleScreenshotsUploadCmd(), newAppleScreenshotsDeleteCmd())
	return cmd
}

func newAppleScreenshotsListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "list [--locale <l>] [--device <d>] [--version <v>]",
		Short: "List the screenshot sets of a version",
		Long: "Lists the screenshot sets per locale and device with their screenshots in order. Without --version:\n" +
			"the editable version if there is one, else the live one. Result: [ScreenshotSet].",
		Example: "  store apple screenshots list --locale en-US --device iphone-6.9",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var device *media.Device
			if alias, _ := cmd.Flags().GetString("device"); alias != "" {
				d, err := apple.DeviceForScreenshots(platformFlag(cmd), alias, nil)
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
			sets, err := svc.ListScreenshots(cmd.Context(), v, locale, device)
			if err != nil {
				return err
			}
			return emit(cmd, sets)
		},
	}
	cmd.Flags().String("locale", "", "only this locale")
	cmd.Flags().String("device", "", "only this device, e.g. iphone-6.9")
	addListingVersionFlag(cmd)
	return cmd
}

func newAppleScreenshotsUploadCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "upload --locale <l> [--device <d>] [--version <v>] [--replace] <png>...",
		Short: "Upload screenshots to the set of one locale and device",
		Long: "Uploads the files in the given order. The device comes from --device or from the pixel size of the\n" +
			"first file (all files must be the same device); an unknown size lists the accepted sizes. Where a\n" +
			"size fits several devices the first one in the device table wins; pass --device to choose. Files are\n" +
			"PNG or JPEG without an alpha channel, checked locally before any API call.\n" +
			"Without --replace the files are appended to the set (max 10 per set). With --replace: if the set\n" +
			"already holds exactly these files in this order (MD5), nothing happens; otherwise the set is emptied\n" +
			"and the files are uploaded in order. Each file is reserved, uploaded, committed with its MD5 and\n" +
			"polled until Apple processed it; --timeout is the time per file. Result: ScreenshotSet; with\n" +
			"--dry-run the planned change.",
		Example: "  store apple screenshots upload --locale en-US --replace shots/en-US/*.png\n" +
			"  store apple screenshots upload --locale de-DE --device ipad-12.9 --timeout 20m ipad1.png ipad2.png",
		Args: cobra.MinimumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireFlags(cmd, "locale"); err != nil {
				return err
			}
			timeout, _ := cmd.Flags().GetDuration("timeout")
			if timeout <= 0 {
				return clierr.Usagef("invalid --timeout %s (must be positive)", timeout)
			}
			alias, _ := cmd.Flags().GetString("device")
			d, err := apple.DeviceForScreenshots(platformFlag(cmd), alias, args)
			if err != nil {
				return err
			}
			if err := apple.CheckScreenshotFiles(d, args); err != nil {
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
			set, change, err := svc.UploadScreenshots(cmd.Context(), v, apple.ScreenshotUpload{
				Locale: locale, Device: d, Paths: args, Replace: replace, Timeout: timeout, DryRun: dry,
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
	cmd.Flags().String("locale", "", "locale of the set, e.g. en-US (required)")
	cmd.Flags().String("device", "", "device of the set, e.g. iphone-6.9 (default: detected from the first file)")
	cmd.Flags().Bool("replace", false, "make the set hold exactly these files instead of appending")
	cmd.Flags().Duration("timeout", 10*time.Minute, "how long one file may take until Apple processed it, e.g. 90s, 10m, 1h")
	addListingVersionFlag(cmd)
	addDryRun(cmd)
	return cmd
}

func newAppleScreenshotsDeleteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "delete --locale <l> --device <d> [--version <v>]",
		Short: "Delete the screenshot set of one locale and device",
		Long: "Deletes the whole set. A set that does not exist is an error. Result: {locale, device, deleted};\n" +
			"with --dry-run the planned change.",
		Example: "  store apple screenshots delete --locale en-US --device iphone-6.5",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireFlags(cmd, "locale", "device"); err != nil {
				return err
			}
			alias, _ := cmd.Flags().GetString("device")
			d, err := apple.DeviceForScreenshots(platformFlag(cmd), alias, nil)
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
			res, change, err := svc.DeleteScreenshots(cmd.Context(), v, locale, d, dry)
			if err != nil {
				return err
			}
			if dry {
				return emit(cmd, metadata.Plan{Changes: []metadata.Change{change}})
			}
			return emit(cmd, res)
		},
	}
	cmd.Flags().String("locale", "", "locale of the set, e.g. en-US (required)")
	cmd.Flags().String("device", "", "device of the set, e.g. iphone-6.9 (required)")
	addListingVersionFlag(cmd)
	addDryRun(cmd)
	return cmd
}

// requireFlags makes a missing or empty required flag a usage error (exit 2),
// which cobra's own required-flag check does not. A repeatable flag must be
// given at least once.
func requireFlags(cmd *cobra.Command, names ...string) error {
	for _, n := range names {
		set := false
		if f := cmd.Flags().Lookup(n); f != nil {
			if sv, ok := f.Value.(pflag.SliceValue); ok {
				set = len(sv.GetSlice()) > 0
			} else {
				set = f.Value.String() != ""
			}
		}
		if !set {
			return clierr.Usagef("required flag --%s not set", n)
		}
	}
	return nil
}
