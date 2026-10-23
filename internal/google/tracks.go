package google

import (
	"context"
	"errors"
	"fmt"
	"io"
	"maps"
	"slices"
	"strconv"
	"strings"

	"google.golang.org/api/androidpublisher/v3"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/locale"
	"github.com/nikeee/store-cli/internal/output"
)

// readOnly is how read commands run: an edit that is created and deleted.
var readOnly = CommitOptions{DryRun: true}

// errNoChange ends an edit without committing it: there is nothing to change.
var errNoChange = errors.New("no change")

// Tracks is the result of ListTracks.
type Tracks []Track

// ListTracks returns every track of the app with its releases.
func (s *Service) ListTracks(ctx context.Context) (Tracks, error) {
	out := Tracks{}
	err := s.withEdit(ctx, readOnly, func(editID string) error {
		l, err := s.api.Edits.Tracks.List(s.packageName, editID).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("list tracks: %w", err)
		}
		for _, t := range l.Tracks {
			out = append(out, trackFromAPI(t))
		}
		return nil
	})
	return out, err
}

// ShowTrack returns one track with its releases.
func (s *Service) ShowTrack(ctx context.Context, track string) (Track, error) {
	var out Track
	err := s.withEdit(ctx, readOnly, func(editID string) error {
		t, err := s.getTrack(ctx, editID, track)
		if err != nil {
			return err
		}
		out = trackFromAPI(t)
		return nil
	})
	return out, err
}

// PromoteOptions are the inputs of Promote.
type PromoteOptions struct {
	Name         string // source release name; empty: the newest by version code
	From, To     string
	Status       string  // API spelling of the target release
	UserFraction float64 // for inProgress and halted
	// Notes (App Store style locale -> text) replace the notes of the source
	// release; nil copies them.
	Notes  map[string]string
	Commit CommitOptions
}

// Promote copies a release from one track to another and returns the target
// track. A target that already has the release's version codes as completed
// is left alone: nothing is changed or committed.
func (s *Service) Promote(ctx context.Context, o PromoteOptions) (Track, error) {
	var res Track
	err := s.withEdit(ctx, o.Commit, func(editID string) error {
		src, err := s.getTrack(ctx, editID, o.From)
		if err != nil {
			return err
		}
		from, err := sourceRelease(src, o.Name)
		if err != nil {
			return err
		}
		dst, err := s.getTrack(ctx, editID, o.To)
		if err != nil {
			return err
		}
		if hasCompleted(dst, from.VersionCodes) {
			s.report.Progressf("release %s is already completed on %s, nothing to do", releaseLabel(from), o.To)
			res = trackFromAPI(dst)
			return errNoChange
		}
		notes := o.Notes
		if notes == nil {
			notes = map[string]string{}
			for _, n := range from.ReleaseNotes {
				notes[locale.FromPlay(n.Language)] = n.Text
			}
		}
		localized, err := s.releaseNotes(ctx, editID, notes)
		if err != nil {
			return err
		}
		rel := &androidpublisher.TrackRelease{
			Name: from.Name, VersionCodes: from.VersionCodes, Status: o.Status,
			UserFraction: o.UserFraction, ReleaseNotes: localized,
		}
		res, err = s.setRelease(ctx, editID, o.To, rel, o.Commit.DryRun)
		return err
	})
	if errors.Is(err, errNoChange) {
		return res, nil
	}
	return res, err
}

// SetRollout sets the user fraction of the track's staged release.
func (s *Service) SetRollout(ctx context.Context, track string, fraction float64, c CommitOptions) (Track, error) {
	t, _, err := s.updateRelease(ctx, track, c, pickStatus(StatusInProgress), nil, func(r *androidpublisher.TrackRelease) {
		r.UserFraction = fraction
	})
	return t, err
}

// Halt halts the track's staged release.
func (s *Service) Halt(ctx context.Context, track string, c CommitOptions) (Track, error) {
	t, _, err := s.updateRelease(ctx, track, c, pickStatus(StatusInProgress), nil, func(r *androidpublisher.TrackRelease) {
		r.Status = StatusHalted
	})
	return t, err
}

// Resume resumes the track's halted release.
func (s *Service) Resume(ctx context.Context, track string, c CommitOptions) (Track, error) {
	t, _, err := s.updateRelease(ctx, track, c, pickStatus(StatusHalted), nil, func(r *androidpublisher.TrackRelease) {
		r.Status = StatusInProgress
	})
	return t, err
}

// CompleteRollout rolls the track's staged (or halted) release out to
// everyone.
func (s *Service) CompleteRollout(ctx context.Context, track string, c CommitOptions) (Track, error) {
	var res Track
	err := s.withEdit(ctx, c, func(editID string) error {
		cur, err := s.getTrack(ctx, editID, track)
		if err != nil {
			return err
		}
		i, err := pickStatus(StatusInProgress, StatusHalted)(track, cur)
		if err != nil {
			return err
		}
		rel := *cur.Releases[i]
		rel.Status, rel.UserFraction = StatusCompleted, 0
		res, err = s.setRelease(ctx, editID, track, &rel, c.DryRun)
		return err
	})
	return res, err
}

// SetReleaseNotes sets the notes (App Store style locales) of a release on
// the track, by name or, with an empty name, the newest by version code. The
// notes of other languages stay. The other releases of the track are not
// touched. Notes for none of the listing's languages are an error.
func (s *Service) SetReleaseNotes(ctx context.Context, track, name string, notes map[string]string, c CommitOptions) (Release, error) {
	var withNotes []*androidpublisher.LocalizedText
	t, i, err := s.updateRelease(ctx, track, c, pickName(name), func(editID string) error {
		var err error
		withNotes, err = s.releaseNotes(ctx, editID, notes)
		if err == nil && len(withNotes) == 0 {
			err = clierr.New("the Play listing has none of the release notes languages %s", strings.Join(slices.Sorted(maps.Keys(notes)), ", "))
		}
		return err
	}, func(r *androidpublisher.TrackRelease) {
		r.ReleaseNotes = mergeNotes(r.ReleaseNotes, withNotes)
	})
	if err != nil {
		return Release{}, err
	}
	return t.Releases[i], nil
}

// mergeNotes returns old with the languages of upd replaced or added, sorted
// by language.
func mergeNotes(old, upd []*androidpublisher.LocalizedText) []*androidpublisher.LocalizedText {
	byLang := map[string]*androidpublisher.LocalizedText{}
	for _, n := range old {
		byLang[n.Language] = n
	}
	for _, n := range upd {
		byLang[n.Language] = n
	}
	var out []*androidpublisher.LocalizedText
	for _, l := range slices.Sorted(maps.Keys(byLang)) {
		out = append(out, byLang[l])
	}
	return out
}

// picker returns the index of the release to change.
type picker func(track string, t *androidpublisher.Track) (int, error)

// pickStatus picks the release with one of the statuses (the first status
// that matches wins). An empty result is an error that says what is missing.
func pickStatus(statuses ...string) picker {
	return func(track string, t *androidpublisher.Track) (int, error) {
		for _, st := range statuses {
			for i, r := range t.Releases {
				if r.Status == st {
					return i, nil
				}
			}
		}
		what := "staged"
		if statuses[0] == StatusHalted {
			what = "halted"
		}
		return 0, clierr.New("track %s has no %s release (status %s)", track, what, strings.Join(statuses, " or "))
	}
}

// pickName picks the release called name, or the newest one if name is empty.
func pickName(name string) picker {
	return func(track string, t *androidpublisher.Track) (int, error) {
		i := newestRelease(t, name)
		if i < 0 {
			return 0, noReleaseError(track, name, t)
		}
		return i, nil
	}
}

// updateRelease changes one release of the track in place, in its own edit.
// prepare, if not nil, runs inside the edit before the track is changed. The
// releases other than the picked one are sent unchanged.
func (s *Service) updateRelease(ctx context.Context, track string, c CommitOptions, pick picker, prepare func(editID string) error, mutate func(*androidpublisher.TrackRelease)) (Track, int, error) {
	var res Track
	var idx int
	err := s.withEdit(ctx, c, func(editID string) error {
		cur, err := s.getTrack(ctx, editID, track)
		if err != nil {
			return err
		}
		if idx, err = pick(track, cur); err != nil {
			return err
		}
		if prepare != nil {
			if err := prepare(editID); err != nil {
				return err
			}
		}
		rel := *cur.Releases[idx]
		mutate(&rel)
		releases := slices.Clone(cur.Releases)
		releases[idx] = &rel
		res, err = s.putTrack(ctx, editID, track, releases, c.DryRun)
		return err
	})
	return res, idx, err
}

// newestRelease returns the index of the release with the highest version
// code (among those called name, if name is set), or -1.
func newestRelease(t *androidpublisher.Track, name string) int {
	best, bestCode := -1, int64(-1)
	for i, r := range t.Releases {
		if name != "" && r.Name != name {
			continue
		}
		code := int64(0)
		for _, v := range r.VersionCodes {
			code = max(code, v)
		}
		if code > bestCode {
			best, bestCode = i, code
		}
	}
	return best
}

func sourceRelease(t *androidpublisher.Track, name string) (*androidpublisher.TrackRelease, error) {
	i := newestRelease(t, name)
	if i < 0 {
		return nil, noReleaseError(t.Track, name, t)
	}
	return t.Releases[i], nil
}

func noReleaseError(track, name string, t *androidpublisher.Track) error {
	if len(t.Releases) == 0 {
		return clierr.New("track %s has no releases", track)
	}
	var names []string
	for _, r := range t.Releases {
		names = append(names, releaseLabel(r))
	}
	return clierr.New("track %s has no release named %q (releases: %s)", track, name, strings.Join(names, ", "))
}

// releaseLabel is a release's name, or its version codes if it has none.
func releaseLabel(r *androidpublisher.TrackRelease) string {
	if r.Name != "" {
		return r.Name
	}
	var codes []string
	for _, v := range r.VersionCodes {
		codes = append(codes, strconv.FormatInt(v, 10))
	}
	return "version codes " + strings.Join(codes, ",")
}

// hasCompleted reports whether the track has a completed release with all of
// codes.
func hasCompleted(t *androidpublisher.Track, codes []int64) bool {
	for _, r := range t.Releases {
		if r.Status == StatusCompleted && len(codes) > 0 && !slices.ContainsFunc(codes, func(c int64) bool { return !slices.Contains(r.VersionCodes, c) }) {
			return true
		}
	}
	return false
}

// RenderText implements output.TextRenderer.
func (l Tracks) RenderText(w io.Writer, s output.Styles) error {
	for i, t := range l {
		if i > 0 {
			if _, err := io.WriteString(w, "\n"); err != nil {
				return err
			}
		}
		if err := t.RenderText(w, s); err != nil {
			return err
		}
	}
	return nil
}

// RenderText implements output.TextRenderer: the track name, then one block
// per release.
func (t Track) RenderText(w io.Writer, s output.Styles) error {
	var sb strings.Builder
	sb.WriteString(s.Key.Render("track:") + " " + t.Track + "\n")
	if len(t.Releases) == 0 {
		sb.WriteString("  " + s.Dim.Render("no releases") + "\n")
	}
	for _, r := range t.Releases {
		sb.WriteString("  " + s.Key.Render("release:") + " " + r.label() + "\n")
		r.writeDetails(&sb, s, "    ")
	}
	_, err := io.WriteString(w, sb.String())
	return err
}

// RenderText implements output.TextRenderer.
func (r Release) RenderText(w io.Writer, s output.Styles) error {
	var sb strings.Builder
	sb.WriteString(s.Key.Render("release:") + " " + r.label() + "\n")
	r.writeDetails(&sb, s, "  ")
	_, err := io.WriteString(w, sb.String())
	return err
}

func (r Release) label() string {
	if r.Name == "" {
		return "(unnamed)"
	}
	return r.Name
}

func (r Release) writeDetails(sb *strings.Builder, s output.Styles, indent string) {
	field := func(k, v string) { sb.WriteString(strings.TrimRight(indent+s.Key.Render(k+":")+" "+v, " ") + "\n") }
	field("status", r.Status)
	field("versionCodes", strings.Join(r.VersionCodes, ", "))
	if r.UserFraction > 0 {
		field("userFraction", strconv.FormatFloat(r.UserFraction, 'g', -1, 64))
	}
	if len(r.ReleaseNotes) > 0 {
		field("releaseNotes", "")
		for _, n := range r.ReleaseNotes {
			sb.WriteString(indent + "  " + n.Locale + ": " + strings.ReplaceAll(n.Text, "\n", "\n"+indent+"    ") + "\n")
		}
	}
}
