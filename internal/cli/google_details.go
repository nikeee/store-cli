package cli

import (
	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/google"
	"github.com/nikeee/store-cli/internal/metadata"
)

func newGoogleDetailsCmd(Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "details",
		Short: "Show or change the app contact details",
	}
	makeGroup(cmd)
	cmd.AddCommand(newGoogleDetailsShowCmd(), newGoogleDetailsSetCmd())
	return cmd
}

func newGoogleDetailsShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "show",
		Short:   "Show the app contact details",
		Long:    "Shows the contact email, website, phone and the default language.\nResult: Details",
		Example: "  store google details show",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := googleService(cmd)
			if err != nil {
				return err
			}
			res, err := svc.ShowDetails(cmd.Context())
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
}

func newGoogleDetailsSetCmd() *cobra.Command {
	flags := map[string]*TextFlag{}
	cmd := &cobra.Command{
		Use:   "set [--email <s>] [--website <s>] [--phone <s>] [--default-locale <l>]",
		Short: "Change the app contact details",
		Long: "Changes the given fields; fields not given stay as they are. An empty value clears a field. A text\n" +
			"starting with @ is read from that file. --default-locale takes an App Store style locale and is\n" +
			"sent as the Play language code. With --dry-run the details are shown as they would be.\n" +
			"Result: Details",
		Example: "  store google details set --email support@example.com --website https://example.com\n" +
			"  store google details set --default-locale en-US",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			var p google.DetailsPatch
			for name, dst := range map[string]**string{"email": &p.Email, "website": &p.Website, "phone": &p.Phone} {
				if tf := flags[name]; tf.Changed {
					v := tf.Value
					*dst = &v
				}
			}
			if cmd.Flags().Changed("default-locale") {
				loc, _ := cmd.Flags().GetString("default-locale")
				if !metadata.IsLocale(loc) {
					return clierr.Usagef("invalid --default-locale %q", loc)
				}
				p.DefaultLocale = &loc
			}
			if p.Email == nil && p.Website == nil && p.Phone == nil && p.DefaultLocale == nil {
				return clierr.Usagef("no field given: pass at least one of --email, --website, --phone, --default-locale")
			}
			svc, err := googleService(cmd)
			if err != nil {
				return err
			}
			res, err := svc.SetDetails(cmd.Context(), p, commitOptions(cmd))
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
	f := cmd.Flags()
	for _, fl := range []struct{ name, usage string }{
		{"email", "contact email address"}, {"website", "contact website"}, {"phone", "contact phone number"},
	} {
		flags[fl.name] = NewTextFlag(fl.name, func(p string) ([]byte, error) { return cmdDeps(cmd).ReadFile(p) })
		f.Var(flags[fl.name], fl.name, fl.usage+", or @file to read it from a file")
	}
	f.String("default-locale", "", "default language of the listing, e.g. en-US")
	addChangesNotSentForReview(cmd)
	addDryRun(cmd)
	return cmd
}
