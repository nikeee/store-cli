package cli

import (
	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/apple"
	"github.com/nikeee/store-cli/internal/clierr"
)

func newAppleTestflightGroupCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "group",
		Short: "Manage TestFlight beta groups",
	}
	makeGroup(cmd)
	cmd.AddCommand(newAppleTestflightGroupListCmd(), newAppleTestflightGroupCreateCmd(), newAppleTestflightGroupAddBuildCmd())
	return cmd
}

func newAppleTestflightGroupListCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "list",
		Short:   "List the beta groups of the app",
		Example: "  store apple testflight group list",
		Long:    "Lists the beta groups of the app.\nResult: [BetaGroup]",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			groups, err := svc.ListGroups(cmd.Context(), app)
			if err != nil {
				return err
			}
			return emit(cmd, groups)
		},
	}
}

func newAppleTestflightGroupCreateCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "create <name>",
		Short:   "Create a beta group",
		Long:    "Creates a beta group, optionally internal or with a public link.\nResult: BetaGroup",
		Example: "  store apple testflight group create Friends --public-link --public-link-limit 100",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			internal, _ := cmd.Flags().GetBool("internal")
			link, _ := cmd.Flags().GetBool("public-link")
			limit, _ := cmd.Flags().GetInt("public-link-limit")
			if limit < 0 {
				return clierr.Usagef("invalid --public-link-limit %d (must be positive)", limit)
			}
			if limit > 0 && !link {
				return clierr.Usagef("--public-link-limit needs --public-link")
			}
			if internal && link {
				return clierr.Usagef("--internal and --public-link cannot be combined")
			}
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			g, err := svc.CreateGroup(cmd.Context(), app, apple.GroupSpec{Name: args[0], Internal: internal, PublicLink: link, PublicLinkLimit: limit}, dryRun(cmd))
			if err != nil {
				return err
			}
			return emit(cmd, g)
		},
	}
	cmd.Flags().Bool("internal", false, "create an internal group")
	cmd.Flags().Bool("public-link", false, "enable the public link")
	cmd.Flags().Int("public-link-limit", 0, "maximum number of testers joining through the public link")
	addDryRun(cmd)
	return cmd
}

func newAppleTestflightGroupAddBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add-build <group> <build>",
		Short: "Add a build to a beta group",
		Long: "Adds a build (number or latest) to a beta group, given by name or ID. External groups need a build\n" +
			"that passed Beta App Review (see testflight submit).\n" +
			"Result: GroupBuild",
		Example: "  store apple testflight group add-build Beta latest",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			g, err := svc.ResolveGroup(cmd.Context(), app, args[0])
			if err != nil {
				return err
			}
			b, err := svc.ResolveBuild(cmd.Context(), app, args[1], versionFlag(cmd), platformFlag(cmd))
			if err != nil {
				return err
			}
			res, err := svc.AddBuildToGroup(cmd.Context(), g, b, dryRun(cmd))
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
	addBuildVersionFlag(cmd)
	addDryRun(cmd)
	return cmd
}

func newAppleTestflightTesterCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "tester",
		Short: "Manage TestFlight testers",
	}
	makeGroup(cmd)
	cmd.AddCommand(newAppleTestflightTesterListCmd(), newAppleTestflightTesterAddCmd(), newAppleTestflightTesterRemoveCmd())
	return cmd
}

func newAppleTestflightTesterListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List the testers of the app",
		Example: "  store apple testflight tester list --group Beta",
		Long:    "Lists the testers of the app, optionally only those of one group.\nResult: [Tester]",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			groupID := ""
			if ref, _ := cmd.Flags().GetString("group"); ref != "" {
				g, err := svc.ResolveGroup(cmd.Context(), app, ref)
				if err != nil {
					return err
				}
				groupID = g.ID
			}
			testers, err := svc.ListTesters(cmd.Context(), app, groupID)
			if err != nil {
				return err
			}
			return emit(cmd, testers)
		},
	}
	cmd.Flags().String("group", "", "only testers of this group (name or ID)")
	return cmd
}

func newAppleTestflightTesterAddCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "add <email>",
		Short: "Add a tester to a beta group",
		Long: "Creates the tester if needed and adds them to the group; already a member is a no-op.\n" +
			"Result: Tester",
		Example: "  store apple testflight tester add ada@example.com --group Beta --first-name Ada",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			first, _ := cmd.Flags().GetString("first-name")
			last, _ := cmd.Flags().GetString("last-name")
			svc, g, err := testerGroup(cmd)
			if err != nil {
				return err
			}
			t, err := svc.AddTester(cmd.Context(), g, args[0], first, last, dryRun(cmd))
			if err != nil {
				return err
			}
			return emit(cmd, t)
		},
	}
	addGroupFlag(cmd)
	cmd.Flags().String("first-name", "", "first name of a new tester")
	cmd.Flags().String("last-name", "", "last name of a new tester")
	addDryRun(cmd)
	return cmd
}

func newAppleTestflightTesterRemoveCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "remove <email>",
		Short:   "Remove a tester from a beta group",
		Long:    "Removes the tester from the group only.\nResult: Tester",
		Example: "  store apple testflight tester remove ada@example.com --group Beta",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, g, err := testerGroup(cmd)
			if err != nil {
				return err
			}
			t, err := svc.RemoveTester(cmd.Context(), g, args[0], dryRun(cmd))
			if err != nil {
				return err
			}
			return emit(cmd, t)
		},
	}
	addGroupFlag(cmd)
	addDryRun(cmd)
	return cmd
}

func addGroupFlag(cmd *cobra.Command) {
	cmd.Flags().String("group", "", "beta group (name or ID)")
}

// testerGroup resolves the required --group flag.
func testerGroup(cmd *cobra.Command) (*apple.Service, apple.BetaGroup, error) {
	ref, _ := cmd.Flags().GetString("group")
	if ref == "" {
		return nil, apple.BetaGroup{}, clierr.Usagef("--group is required")
	}
	svc, app, err := appleService(cmd)
	if err != nil {
		return nil, apple.BetaGroup{}, err
	}
	g, err := svc.ResolveGroup(cmd.Context(), app, ref)
	return svc, g, err
}

func newAppleTestflightSubmitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit <build>",
		Short: "Submit a build to Beta App Review",
		Long: "Submits the build to Beta App Review (needed for external testing). Already submitted is a no-op.\n" +
			"Result: BetaReview",
		Example: "  store apple testflight submit latest",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			b, err := svc.ResolveBuild(cmd.Context(), app, args[0], versionFlag(cmd), platformFlag(cmd))
			if err != nil {
				return err
			}
			res, err := svc.SubmitBetaReview(cmd.Context(), b, dryRun(cmd))
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
	addBuildVersionFlag(cmd)
	addDryRun(cmd)
	return cmd
}
