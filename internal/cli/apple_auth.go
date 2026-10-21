package cli

import "github.com/spf13/cobra"

// appleAuthCheck is the result of `store apple auth check`.
type appleAuthCheck struct {
	IssuerID string `json:"issuerId"`
	KeyID    string `json:"keyId"`
	OK       bool   `json:"ok"`
}

func newAppleAuthCmd(Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Check the App Store Connect credentials",
	}
	makeGroup(cmd)
	cmd.AddCommand(&cobra.Command{
		Use:     "check",
		Short:   "Verify the credentials with one read call",
		Example: "  store apple auth check",
		Long:    "Verifies the credentials with one read call.\n\nResult: {issuerId, keyId, ok}. Never prints the key.",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			creds, err := appleCredentials(cmd)
			if err != nil {
				return err
			}
			svc, err := newAppleService(cmd, creds)
			if err != nil {
				return err
			}
			if err := svc.CheckAuth(cmd.Context()); err != nil {
				return err
			}
			return emit(cmd, appleAuthCheck{IssuerID: creds.IssuerID, KeyID: creds.KeyID, OK: true})
		},
	})
	return cmd
}
