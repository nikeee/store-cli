package cli

import (
	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/metadata"
)

func newAppleReviewInfoCmd(Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "review-info",
		Short: "Show or change the App Review information",
	}
	makeGroup(cmd)
	cmd.AddCommand(newAppleReviewInfoShowCmd(), newAppleReviewInfoSetCmd())
	return cmd
}

func newAppleReviewInfoShowCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "show [--version <v>]",
		Short: "Show the App Review information",
		Long: "Shows the App Review contact, demo account, notes and attachments of a version. Without --version:\n" +
			"the editable version if there is one, else the live one. The demo password is shown as *** unless\n" +
			"--reveal is given. Result: {firstName, lastName, phone, email, demoRequired, demoUser,\n" +
			"demoPassword, notes, attachments: [fileName]}.",
		Example: "  store apple review-info show --reveal",
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
			reveal, _ := cmd.Flags().GetBool("reveal")
			info, err := svc.ShowReviewInfo(cmd.Context(), v, reveal)
			if err != nil {
				return err
			}
			return emit(cmd, info)
		},
	}
	addListingVersionFlag(cmd)
	cmd.Flags().Bool("reveal", false, "show the demo password instead of ***")
	return cmd
}

func newAppleReviewInfoSetCmd() *cobra.Command {
	flags := map[string]*TextFlag{}
	cmd := &cobra.Command{
		Use:   "set [--version <v>] [field flags] [--attachment <file>]...",
		Short: "Change the App Review information",
		Long: "Changes the given fields; fields not given stay as they are. An empty value clears a field, except\n" +
			"--demo-required, which cannot be cleared. A text starting with @ is read from that file.\n" +
			"--attachment (repeatable) replaces all existing attachments with the given files. Result: the\n" +
			"review information as in show (password masked); with --dry-run the planned changes.",
		Example: "  store apple review-info set --demo-required true --demo-user reviewer --demo-password @pw.txt\n" +
			"  store apple review-info set --attachment login-flow.pdf",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			texts := metadata.Texts{}
			for _, f := range metadata.AppleReviewFields {
				if tf := flags[f.Flag]; tf.Changed {
					texts[f.API] = tf.Value
				}
			}
			attachments, _ := cmd.Flags().GetStringArray("attachment")
			if len(texts) == 0 && len(attachments) == 0 {
				return clierr.Usagef("no field given: pass at least one field flag or --attachment (see --help)")
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
			info, changes, err := svc.SetReviewInfo(cmd.Context(), v, texts, attachments, dry)
			if err != nil {
				return err
			}
			if dry {
				return emit(cmd, metadata.Plan{Changes: changes})
			}
			return emit(cmd, info)
		},
	}
	addListingVersionFlag(cmd)
	for _, f := range metadata.AppleReviewFields {
		tf := NewTextFlag(f.Flag, func(p string) ([]byte, error) { return cmdDeps(cmd).ReadFile(p) })
		flags[f.Flag] = tf
		cmd.Flags().Var(tf, f.Flag, f.Help+", or @file to read it from a file")
	}
	cmd.Flags().StringArray("attachment", nil, "attachment file for App Review; replaces all existing attachments (repeatable)")
	addDryRun(cmd)
	return cmd
}
