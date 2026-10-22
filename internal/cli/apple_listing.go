package cli

import (
	"context"

	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/apple"
	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/metadata"
)

func newAppleListingCmd(Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "listing",
		Short: "Show and change the store listing texts",
	}
	makeGroup(cmd)
	cmd.AddCommand(newAppleListingShowCmd(), newAppleListingSetCmd(), newAppleListingLocaleCmd())
	return cmd
}

func addListingVersionFlag(cmd *cobra.Command) {
	cmd.Flags().String("version", "", "app version (default: the editable version, else the live one)")
}

// listingVersion returns the version given by --version, else the current one.
func listingVersion(cmd *cobra.Command, svc *apple.Service, app apple.App) (apple.Version, error) {
	if v := versionFlag(cmd); v != "" {
		return findVersion(cmd, svc, app, v)
	}
	return svc.CurrentVersion(cmd.Context(), app, platformFlag(cmd))
}

func newAppleListingShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show the listing texts per locale",
		Long: "Shows the app info and version texts per locale. Without --version: the editable version if there\n" +
			"is one, else the live one.\n" +
			"Result: [Localization]",
		Example: "  store apple listing show --locale de-DE",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			v, err := listingVersion(cmd, svc, app)
			if err != nil {
				return err
			}
			locale, _ := cmd.Flags().GetString("locale")
			ls, err := svc.ShowListing(cmd.Context(), app, v, locale)
			if err != nil {
				return err
			}
			return emit(cmd, ls)
		},
	}
	cmd.Flags().String("locale", "", "only this locale")
	addListingVersionFlag(cmd)
	return cmd
}

func newAppleListingSetCmd() *cobra.Command {
	flags := map[string]*TextFlag{}
	cmd := &cobra.Command{
		Use:   "set --locale <l> [--version <v>] [field flags]",
		Short: "Change listing texts of one locale",
		Long: "Changes the given fields of one locale; fields not given stay as they are. A locale that does not\n" +
			"exist yet is created. An empty value clears a field. Lengths are checked before any API call.\n" +
			"App info fields (name, subtitle, privacy URLs) need an editable app info. A text starting with @\n" +
			"is read from that file. With --dry-run the result is the planned Plan.\n" +
			"Result: Localization",
		Example: "  store apple listing set --locale en-US --subtitle \"Fast and small\" --keywords @keywords.txt",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireFlags(cmd, "locale"); err != nil {
				return err
			}
			locale, _ := cmd.Flags().GetString("locale")
			texts := metadata.Texts{}
			for _, f := range metadata.AppleFields {
				if tf := flags[f.Flag]; f.Flag != "" && tf.Changed {
					texts[f.API] = tf.Value
				}
			}
			if len(texts) == 0 {
				return clierr.Usagef("no field given: pass at least one field flag (see --help)")
			}
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			v, err := listingVersion(cmd, svc, app)
			if err != nil {
				return err
			}
			dry := dryRun(cmd)
			loc, changes, err := svc.SetListing(cmd.Context(), app, v, locale, texts, dry)
			if err != nil {
				return err
			}
			if dry {
				return emit(cmd, metadata.Plan{Changes: changes})
			}
			return emit(cmd, loc)
		},
	}
	cmd.Flags().String("locale", "", "locale to change, e.g. en-US (required)")
	addListingVersionFlag(cmd)
	for _, f := range metadata.AppleFields {
		if f.Flag == "" {
			continue
		}
		tf := NewTextFlag(f.Flag, func(p string) ([]byte, error) { return cmdDeps(cmd).ReadFile(p) })
		flags[f.Flag] = tf
		cmd.Flags().Var(tf, f.Flag, f.Help+", or @file to read it from a file")
	}
	addDryRun(cmd)
	return cmd
}

func newAppleListingLocaleCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "locale",
		Short: "Add or remove a locale",
	}
	makeGroup(cmd)
	cmd.AddCommand(
		newAppleListingLocaleChangeCmd("add", "Add a locale to the version and the app info",
			"Creates the localization on the version and the app info where it is missing.\nResult: LocaleResult",
			(*apple.Service).AddLocale),
		newAppleListingLocaleChangeCmd("remove", "Remove a locale from the version and the app info",
			"Deletes the localization from the version and the app info. The primary locale cannot be removed.\nResult: LocaleResult",
			(*apple.Service).RemoveLocale),
	)
	return cmd
}

func newAppleListingLocaleChangeCmd(name, short, long string, run func(*apple.Service, context.Context, apple.App, apple.Version, string, bool) (apple.LocaleResult, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:     name + " <locale>",
		Short:   short,
		Long:    long,
		Example: "  store apple listing locale " + name + " de-DE",
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
			res, err := run(svc, cmd.Context(), app, v, args[0], dryRun(cmd))
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
	addListingVersionFlag(cmd)
	addDryRun(cmd)
	return cmd
}
