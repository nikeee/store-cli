package cli

import (
	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/locale"
	"github.com/nikeee/store-cli/internal/metadata"
)

func newGoogleListingCmd(Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "listing",
		Short: "Show and change the store listing texts",
	}
	makeGroup(cmd)
	cmd.AddCommand(newGoogleListingShowCmd(), newGoogleListingSetCmd())
	return cmd
}

// playLocaleFlag returns the Play language code of the App Store style locale
// in --locale; a malformed one is a usage error.
func playLocaleFlag(cmd *cobra.Command) (string, error) {
	loc, _ := cmd.Flags().GetString("locale")
	if !metadata.IsLocale(loc) {
		return "", clierr.Usagef("invalid --locale %q", loc)
	}
	return locale.ToPlay(loc), nil
}

func newGoogleListingShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show",
		Short: "Show the listing texts per locale",
		Long: "Shows the title, short and full description and promo video per listing language. --locale takes an\n" +
			"App Store style locale, sent as the Play language code.\n" +
			"Result: [Listing]",
		Example: "  store google listing show --locale de-DE",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			language := ""
			if cmd.Flags().Changed("locale") {
				var err error
				if language, err = playLocaleFlag(cmd); err != nil {
					return err
				}
			}
			svc, err := googleService(cmd)
			if err != nil {
				return err
			}
			res, err := svc.ShowListings(cmd.Context(), language)
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
	cmd.Flags().String("locale", "", "only this locale")
	return cmd
}

func newGoogleListingSetCmd() *cobra.Command {
	flags := map[string]*TextFlag{}
	cmd := &cobra.Command{
		Use:   "set --locale <l> [field flags]",
		Short: "Change listing texts of one locale",
		Long: "Changes the given fields of one locale; fields not given stay as they are (including the promo video).\n" +
			"A locale that does not exist yet is created. An empty value clears a field. Lengths are checked\n" +
			"before any API call. --locale takes an App Store style locale, sent as the Play language code. A text\n" +
			"starting with @ is read from that file. With --dry-run the listing is shown as it would be.\n" +
			"Result: Listing",
		Example: "  store google listing set --locale en-US --short-description \"Fast and small\" --full-description @full.txt",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			if err := requireFlags(cmd, "locale"); err != nil {
				return err
			}
			language, err := playLocaleFlag(cmd)
			if err != nil {
				return err
			}
			texts := metadata.Texts{}
			for _, f := range metadata.GoogleFields {
				if tf := flags[f.Flag]; f.Flag != "" && tf.Changed {
					texts[f.API] = tf.Value
				}
			}
			if len(texts) == 0 {
				return clierr.Usagef("no field given: pass at least one field flag (see --help)")
			}
			svc, err := googleService(cmd)
			if err != nil {
				return err
			}
			res, err := svc.SetListing(cmd.Context(), language, texts, commitOptions(cmd))
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
	cmd.Flags().String("locale", "", "locale to change, e.g. en-US (required)")
	for _, f := range metadata.GoogleFields {
		if f.Flag == "" {
			continue
		}
		tf := NewTextFlag(f.Flag, func(p string) ([]byte, error) { return cmdDeps(cmd).ReadFile(p) })
		flags[f.Flag] = tf
		cmd.Flags().Var(tf, f.Flag, f.Help+", or @file to read it from a file")
	}
	addChangesNotSentForReview(cmd)
	addDryRun(cmd)
	return cmd
}
