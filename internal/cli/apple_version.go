package cli

import (
	"time"

	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/apple"
	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/clierr"
)

func newAppleVersionCmd(Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "version",
		Short: "Manage App Store versions",
	}
	makeGroup(cmd)
	cmd.AddCommand(
		newAppleVersionListCmd(),
		newAppleVersionShowCmd(),
		newAppleVersionCreateCmd(),
		newAppleVersionSetBuildCmd(),
		newAppleVersionSubmitCmd(),
		newAppleVersionCancelCmd(),
		newAppleVersionReleaseCmd(),
		newAppleVersionPhasedCmd(),
	)
	return cmd
}

func newAppleVersionListCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "list",
		Short:   "List the versions of the app, newest first",
		Example: "  store apple version list --limit 5",
		Long:    "Lists the versions of the app on --platform, newest first. Result: [Version].",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			limit, _ := cmd.Flags().GetInt("limit")
			if limit < 1 || limit > 200 {
				return clierr.Usagef("invalid --limit %d (use 1 to 200)", limit)
			}
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			vs, err := svc.ListVersions(cmd.Context(), app, platformFlag(cmd), limit)
			if err != nil {
				return err
			}
			return emit(cmd, vs)
		},
	}
	cmd.Flags().Int("limit", 10, "maximum number of versions")
	return cmd
}

func newAppleVersionShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "show <version>",
		Short:   "Show one version",
		Example: "  store apple version show 1.4.0",
		Long:    "Result: Version.",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			v, err := findVersion(cmd, svc, app, args[0])
			if err != nil {
				return err
			}
			return emit(cmd, v)
		},
	}
}

// findVersion returns the version v on --platform; a missing one is an error
// that points at version list.
func findVersion(cmd *cobra.Command, svc *apple.Service, app apple.App, v string) (apple.Version, error) {
	found, ok, err := svc.FindVersion(cmd.Context(), app, v, platformFlag(cmd))
	if err != nil {
		return apple.Version{}, err
	}
	if !ok {
		e := clierr.New("version %s not found", v)
		e.Hint = "store apple version list"
		return apple.Version{}, e
	}
	return found, nil
}

func newAppleVersionCreateCmd() *cobra.Command {
	var copyright *TextFlag
	cmd := &cobra.Command{
		Use:   "create <version>",
		Short: "Create a new App Store version",
		Long: "Creates the version. Fails if the version exists. --release scheduled needs --release-date.\n" +
			"Result: Version.",
		Example: "  store apple version create 1.4.0 --release scheduled --release-date 2026-12-01T09:00:00Z",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			o := apple.CreateVersionOptions{Version: args[0], Platform: platformFlag(cmd), DryRun: dryRun(cmd)}
			if r, _ := cmd.Flags().GetString("release"); r != "" {
				t, err := apple.ParseReleaseType(r)
				if err != nil {
					return err
				}
				o.ReleaseType = t
			}
			if d, _ := cmd.Flags().GetString("release-date"); d != "" {
				t, err := time.Parse(time.RFC3339, d)
				if err != nil {
					return clierr.Usagef("invalid --release-date %q (use an RFC 3339 time, e.g. 2026-12-01T09:00:00Z)", d)
				}
				o.ReleaseDate = &t
			}
			if copyright.Changed {
				o.Copyright = &copyright.Value
			}
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			v, err := svc.CreateVersion(cmd.Context(), app, o)
			if err != nil {
				return err
			}
			return emit(cmd, v)
		},
	}
	copyright = NewTextFlag("copyright", func(p string) ([]byte, error) { return cmdDeps(cmd).ReadFile(p) })
	cmd.Flags().String("release", "", "how the version is released: manual, after-approval or scheduled")
	cmd.Flags().String("release-date", "", "release time for --release scheduled, an RFC 3339 time")
	cmd.Flags().Var(copyright, "copyright", "copyright text, or @file to read it from a file")
	addDryRun(cmd)
	return cmd
}

func newAppleVersionSetBuildCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "set-build <version> <build>",
		Short: "Attach a build to an editable version",
		Long: "Attaches a processed (VALID) build to an editable version. The build is looked up under the\n" +
			"version's own number. Result: Version.",
		Example: "  store apple version set-build 1.4.0 45\n  store apple version set-build 1.4.0 latest",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			v, err := findVersion(cmd, svc, app, args[0])
			if err != nil {
				return err
			}
			b, err := svc.ResolveBuild(cmd.Context(), app, args[1], v.Version, platformFlag(cmd))
			if err != nil {
				return err
			}
			if dryRun(cmd) {
				if err := apple.CheckSetVersionBuild(v, b); err != nil {
					return err
				}
				v.Build = &apple.VersionBuild{ID: b.ID, Build: b.Build}
				return emit(cmd, v)
			}
			v, err = svc.SetVersionBuild(cmd.Context(), v, b)
			if err != nil {
				return err
			}
			return emit(cmd, v)
		},
	}
	addDryRun(cmd)
	return cmd
}

// findVersionToChange returns the version v on --platform; a missing one is
// an error that points at version create.
func findVersionToChange(cmd *cobra.Command, svc *apple.Service, app apple.App, v string) (apple.Version, error) {
	found, ok, err := svc.FindVersion(cmd.Context(), app, v, platformFlag(cmd))
	if err != nil {
		return apple.Version{}, err
	}
	if !ok {
		e := clierr.New("version %s not found", v)
		e.Hint = "store apple version create " + v
		return apple.Version{}, e
	}
	return found, nil
}

func newAppleVersionSubmitCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "submit <version>",
		Short: "Submit a version for App Review",
		Long: "Submits the version for App Review. An open, unsubmitted review submission of the app is reused\n" +
			"together with its other items (Apple only accepts e.g. a first subscription group together with an\n" +
			"app version). If that open submission holds a different version, the command fails and names it.\n" +
			"Result: {version, submissionId, state, items}.",
		Example: "  store apple version submit 1.4.0",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			v, err := findVersionToChange(cmd, svc, app, args[0])
			if err != nil {
				return err
			}
			submit := svc.SubmitVersion
			if dryRun(cmd) {
				submit = svc.PreviewSubmitVersion
			}
			res, err := submit(cmd.Context(), app, v)
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
	addDryRun(cmd)
	return cmd
}

func newAppleVersionCancelCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "cancel <version>",
		Short:   "Withdraw the review submission of a version",
		Long:    "Withdraws the version's review submission. Result: {version, submissionId, state}.",
		Example: "  store apple version cancel 1.4.0",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			v, err := findVersionToChange(cmd, svc, app, args[0])
			if err != nil {
				return err
			}
			cancel := svc.CancelVersion
			if dryRun(cmd) {
				cancel = svc.PreviewCancelVersion
			}
			res, err := cancel(cmd.Context(), app, v)
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
	addDryRun(cmd)
	return cmd
}

func newAppleVersionReleaseCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:     "release <version>",
		Short:   "Release a version that waits for the developer release",
		Long:    "Releases a version in PENDING_DEVELOPER_RELEASE. Result: Version.",
		Example: "  store apple version release 1.4.0",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			v, err := findVersionToChange(cmd, svc, app, args[0])
			if err != nil {
				return err
			}
			v, err = svc.ReleaseVersion(cmd.Context(), v, dryRun(cmd))
			if err != nil {
				return err
			}
			return emit(cmd, v)
		},
	}
	addDryRun(cmd)
	return cmd
}

func newAppleVersionPhasedCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "phased",
		Short: "Manage the phased release of a version",
	}
	makeGroup(cmd)
	cmd.AddCommand(
		newAppleVersionPhasedStartCmd(),
		newAppleVersionPhasedStateCmd("pause", asc.PhasedReleaseStatePaused, "Pause the running phased release"),
		newAppleVersionPhasedStateCmd("resume", asc.PhasedReleaseStateActive, "Resume a paused phased release"),
		newAppleVersionPhasedStateCmd("complete", asc.PhasedReleaseStateComplete, "Release to all users now, ending the phased release"),
	)
	return cmd
}

func newAppleVersionPhasedStartCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "start <version>",
		Short: "Enable phased release for a version",
		Long: "Enables phased release for a version that is not live yet. It starts with the release. For a version\n" +
			"that is live it starts at once. A version that has a phased release is left as it is.\n" +
			"Result: {version, state, currentDayNumber}.",
		Example: "  store apple version phased start 1.4.0",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			v, err := findVersionToChange(cmd, svc, app, args[0])
			if err != nil {
				return err
			}
			p, err := svc.StartPhasedRelease(cmd.Context(), v, dryRun(cmd))
			if err != nil {
				return err
			}
			return emit(cmd, p)
		},
	}
	addDryRun(cmd)
	return cmd
}

func newAppleVersionPhasedStateCmd(name string, state asc.PhasedReleaseState, short string) *cobra.Command {
	cmd := &cobra.Command{
		Use:     name + " <version>",
		Short:   short,
		Long:    "Sets the phased release of the version to " + string(state) + ". Result: {version, state, currentDayNumber}.",
		Example: "  store apple version phased " + name + " 1.4.0",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, app, err := appleService(cmd)
			if err != nil {
				return err
			}
			v, err := findVersionToChange(cmd, svc, app, args[0])
			if err != nil {
				return err
			}
			p, err := svc.SetPhasedReleaseState(cmd.Context(), v, state, dryRun(cmd))
			if err != nil {
				return err
			}
			return emit(cmd, p)
		},
	}
	addDryRun(cmd)
	return cmd
}
