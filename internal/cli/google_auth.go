package cli

import "github.com/spf13/cobra"

// GoogleAuthCheck is the result of `store google auth check`.
type GoogleAuthCheck struct {
	ClientEmail string `json:"clientEmail"`
	App         string `json:"app"`
	OK          bool   `json:"ok"`
}

func newGoogleAuthCmd(Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Check the Google Play credentials",
	}
	makeGroup(cmd)
	cmd.AddCommand(&cobra.Command{
		Use:     "check",
		Short:   "Verify the credentials by creating and deleting an edit",
		Example: "  store google auth check --app com.example.app",
		Long:    "Creates and deletes an edit for --app.\nResult: GoogleAuthCheck",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := googleService(cmd)
			if err != nil {
				return err
			}
			if err := svc.CheckAuth(cmd.Context()); err != nil {
				return err
			}
			return emit(cmd, GoogleAuthCheck{ClientEmail: svc.ClientEmail(), App: svc.PackageName(), OK: true})
		},
	})
	return cmd
}
