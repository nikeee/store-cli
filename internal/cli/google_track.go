package cli

import (
	"context"
	"regexp"
	"strconv"

	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/google"
)

func newGoogleTrackCmd(Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "track",
		Short: "Inspect and promote release tracks",
	}
	makeGroup(cmd)
	cmd.AddCommand(
		newGoogleTrackListCmd(), newGoogleTrackShowCmd(), newGoogleTrackPromoteCmd(),
		newGoogleTrackRolloutCmd(), newGoogleTrackTestersCmd(),
		newGoogleTrackStageCmd("halt", "Halt the staged rollout of a track",
			"Halts the track's staged release (status halted, user fraction kept).\nResult: Track",
			func(s *google.Service, ctx context.Context, track string, c google.CommitOptions) (google.Track, error) {
				return s.Halt(ctx, track, c)
			}),
		newGoogleTrackStageCmd("resume", "Resume the halted rollout of a track",
			"Resumes the track's halted release (status in progress, user fraction kept).\nResult: Track",
			func(s *google.Service, ctx context.Context, track string, c google.CommitOptions) (google.Track, error) {
				return s.Resume(ctx, track, c)
			}),
		newGoogleTrackStageCmd("complete", "Roll the staged release of a track out to everyone",
			"Completes the track's staged or halted release (status completed, no user fraction). It replaces\n"+
				"the other releases of the track.\n"+
				"Result: Track",
			func(s *google.Service, ctx context.Context, track string, c google.CommitOptions) (google.Track, error) {
				return s.CompleteRollout(ctx, track, c)
			}),
	)
	return cmd
}

func newGoogleTrackListCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List the tracks and their releases",
		Long: "Lists the tracks: internal, alpha, beta, production and custom closed testing tracks, with their\n" +
			"releases.\n" +
			"Result: [Track]",
		Example: "  store google track list",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := googleService(cmd)
			if err != nil {
				return err
			}
			res, err := svc.ListTracks(cmd.Context())
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
}

func newGoogleTrackShowCmd() *cobra.Command {
	return &cobra.Command{
		Use:     "show <track>",
		Short:   "Show a track and its releases",
		Long:    "Shows one track with its releases.\nResult: Track",
		Example: "  store google track show production",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := googleService(cmd)
			if err != nil {
				return err
			}
			res, err := svc.ShowTrack(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
}

func newGoogleTrackPromoteCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "promote [<release-name>] --from <track> --to <track>",
		Short: "Copy a release from one track to another",
		Long: "Copies a release (by name, default the newest by version code) from --from to --to. The release\n" +
			"becomes completed on the target, or in-progress with --rollout. Release notes come from\n" +
			"--release-notes, else from the source release; languages the listing does not have are dropped\n" +
			"with a warning. A release that is already completed on the target is a no-op. Without a match,\n" +
			"the error lists the release names of the source track. --status in-progress and halted need\n" +
			"--rollout. The result is the target track.\n" +
			"Result: Track",
		Example: "  store google track promote --from internal --to production --rollout 0.1\n" +
			"  store google track promote v1.4.0 --from beta --to production",
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireFlags(cmd, "from", "to"); err != nil {
				return err
			}
			f := cmd.Flags()
			o := google.PromoteOptions{Commit: commitOptions(cmd)}
			o.From, _ = f.GetString("from")
			o.To, _ = f.GetString("to")
			if len(args) == 1 {
				o.Name = args[0]
			}
			var err error
			if o.Status, o.UserFraction, err = releaseStatus(cmd); err != nil {
				return err
			}
			if dir, _ := f.GetString("release-notes"); dir != "" {
				if o.Notes, err = google.ReadReleaseNotesDir(dir); err != nil {
					return err
				}
				if err := google.ValidateReleaseNotes(o.Notes); err != nil {
					return err
				}
			}
			svc, err := googleService(cmd)
			if err != nil {
				return err
			}
			res, err := svc.Promote(cmd.Context(), o)
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
	f := cmd.Flags()
	f.String("from", "", "source track")
	f.String("to", "", "target track")
	f.String("status", "completed", "target release status: completed, draft, in-progress or halted")
	f.Float64("rollout", 0, "staged rollout fraction, greater than 0 and less than 1; implies --status in-progress")
	f.String("release-notes", "", "directory with one <locale>.txt per locale; default: the notes of the source release")
	addChangesNotSentForReview(cmd)
	addDryRun(cmd)
	return cmd
}

func newGoogleTrackRolloutCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "rollout <track> <fraction>",
		Short: "Change the rollout fraction of a track",
		Long: "Sets the user fraction of the track's staged (in progress) release. The fraction must be greater\n" +
			"than 0 and less than 1; use `track complete` to reach everyone. A track without a staged release\n" +
			"is an error.\n" +
			"Result: Track",
		Example: "  store google track rollout production 0.5",
		Args:    cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			fraction, err := strconv.ParseFloat(args[1], 64)
			if err != nil || !(fraction > 0 && fraction < 1) {
				return clierr.Usagef("invalid fraction %q (must be a number greater than 0 and less than 1)", args[1])
			}
			svc, err := googleService(cmd)
			if err != nil {
				return err
			}
			res, err := svc.SetRollout(cmd.Context(), args[0], fraction, commitOptions(cmd))
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
	addChangesNotSentForReview(cmd)
	addDryRun(cmd)
	return cmd
}

// newGoogleTrackStageCmd builds halt, resume and complete: commands that take
// a track and act on its staged release.
func newGoogleTrackStageCmd(name, short, long string, run func(*google.Service, context.Context, string, google.CommitOptions) (google.Track, error)) *cobra.Command {
	cmd := &cobra.Command{
		Use:     name + " <track>",
		Short:   short,
		Long:    long,
		Example: "  store google track " + name + " production",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := googleService(cmd)
			if err != nil {
				return err
			}
			res, err := run(svc, cmd.Context(), args[0], commitOptions(cmd))
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
	addChangesNotSentForReview(cmd)
	addDryRun(cmd)
	return cmd
}

// groupEmailRe is a loose shape check for a Google Group address.
var groupEmailRe = regexp.MustCompile(`^[^@\s]+@[^@\s]+\.[^@\s]+$`)

func newGoogleTrackTestersCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "testers",
		Short: "Show or set the tester groups of a track",
	}
	makeGroup(cmd)
	show := &cobra.Command{
		Use:     "show <track>",
		Short:   "Show the tester groups of a track",
		Long:    "Shows the Google Groups that test a track.\nResult: Testers",
		Example: "  store google track testers show internal",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			svc, err := googleService(cmd)
			if err != nil {
				return err
			}
			res, err := svc.ShowTesters(cmd.Context(), args[0])
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
	set := &cobra.Command{
		Use:   "set <track> --group <google-group-email>...",
		Short: "Replace the tester groups of a track",
		Long: "Replaces the Google Groups that test a track with the given --group addresses (repeatable, at\n" +
			"least one). With --dry-run the result shows what would be stored.\n" +
			"Result: Testers",
		Example: "  store google track testers set internal --group qa@googlegroups.com --group beta@example.com",
		Args:    cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := requireFlags(cmd, "group"); err != nil {
				return err
			}
			groups := stringArray(cmd.Flags(), "group")
			for _, g := range groups {
				if !groupEmailRe.MatchString(g) {
					return clierr.Usagef("invalid --group %q (expected a Google Group email address)", g)
				}
			}
			svc, err := googleService(cmd)
			if err != nil {
				return err
			}
			res, err := svc.SetTesters(cmd.Context(), args[0], groups, commitOptions(cmd))
			if err != nil {
				return err
			}
			return emit(cmd, res)
		},
	}
	set.Flags().StringArray("group", nil, "Google Group email address of the testers (repeatable)")
	addChangesNotSentForReview(set)
	addDryRun(set)
	cmd.AddCommand(show, set)
	return cmd
}
