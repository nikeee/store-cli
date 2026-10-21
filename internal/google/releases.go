package google

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"unicode/utf8"

	"google.golang.org/api/androidpublisher/v3"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/locale"
)

// maxReleaseNoteRunes is Play's limit for the release notes of one language.
const maxReleaseNoteRunes = 500

// Release statuses as the CLI accepts them and as the API spells them.
const (
	StatusCompleted  = "completed"
	StatusDraft      = "draft"
	StatusInProgress = "inProgress"
	StatusHalted     = "halted"
)

// Track is a release track with its releases.
type Track struct {
	Track    string    `json:"track"`
	Releases []Release `json:"releases"`
}

// Release is one release on a track; Status uses the API spelling.
type Release struct {
	Name         string        `json:"name"`
	VersionCodes []string      `json:"versionCodes"`
	Status       string        `json:"status"`
	UserFraction float64       `json:"userFraction,omitempty"`
	ReleaseNotes []ReleaseNote `json:"releaseNotes"`
}

// ReleaseNote is the "What's new" text of one language.
type ReleaseNote struct {
	Locale string `json:"locale"`
	Text   string `json:"text"`
}

// ValidateReleaseNotes checks the limit of 500 characters (runes) per
// language, locally and before any request.
func ValidateReleaseNotes(notes map[string]string) error {
	for _, l := range slices.Sorted(mapKeys(notes)) {
		if n := utf8.RuneCountInString(notes[l]); n > maxReleaseNoteRunes {
			return clierr.New("release notes %s: %d characters, the limit is %d", l, n, maxReleaseNoteRunes)
		}
	}
	return nil
}

// releaseNotes maps notes (App Store style locales) to Play languages. A
// language the app's Play listing does not have is dropped with a warning,
// since Play rejects the whole request for it. The result is sorted by
// language.
func (s *Service) releaseNotes(ctx context.Context, editID string, notes map[string]string) ([]*androidpublisher.LocalizedText, error) {
	if err := ValidateReleaseNotes(notes); err != nil {
		return nil, err
	}
	if len(notes) == 0 {
		return nil, nil
	}
	listings, err := s.api.Edits.Listings.List(s.packageName, editID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("list store listings: %w", err)
	}
	have := map[string]bool{}
	for _, l := range listings.Listings {
		have[l.Language] = true
	}
	byLang := map[string]string{}
	for _, l := range slices.Sorted(mapKeys(notes)) {
		lang := locale.ToPlay(l)
		if !have[lang] {
			s.report.Warnf("release notes %s dropped: the Play listing has no language %s", l, lang)
			continue
		}
		byLang[lang] = notes[l]
	}
	var out []*androidpublisher.LocalizedText
	for _, lang := range slices.Sorted(mapKeys(byLang)) {
		out = append(out, &androidpublisher.LocalizedText{Language: lang, Text: byLang[lang]})
	}
	return out, nil
}

// setRelease puts rel on track t of the edit and returns the track as
// stored. Edits.Tracks.Update replaces the whole release list, so the
// releases that stay are decided here.
//
// Rule, from https://developers.google.com/android-publisher/tracks (the page
// does not state a general rule; it says an inProgress release can sit on top
// of a completed one, and that a halted or staged release falls back to the
// previous completed release):
//   - completed: the new release replaces every existing release, a rollout
//     to everyone supersedes staged and draft releases;
//   - inProgress or halted: the existing completed release stays beside it
//     as the fallback; existing staged and draft releases are replaced;
//   - draft: every existing release that is not a draft stays; an existing
//     draft is replaced (a track holds one draft).
//
// With dryRun the track is read and the result computed, nothing is updated.
func (s *Service) setRelease(ctx context.Context, editID, track string, rel *androidpublisher.TrackRelease, dryRun bool) (Track, error) {
	cur, err := s.api.Edits.Tracks.Get(s.packageName, editID, track).Context(ctx).Do()
	if err != nil {
		return Track{}, fmt.Errorf("read track %s: %w", track, err)
	}
	var kept []*androidpublisher.TrackRelease
	for _, r := range cur.Releases {
		switch rel.Status {
		case StatusCompleted:
		case StatusDraft:
			if r.Status != StatusDraft {
				kept = append(kept, r)
			}
		default:
			if r.Status == StatusCompleted {
				kept = append(kept, r)
			}
		}
	}
	out := &androidpublisher.Track{Track: track, Releases: append(kept, rel)}
	if !dryRun {
		if out, err = s.api.Edits.Tracks.Update(s.packageName, editID, track, out).Context(ctx).Do(); err != nil {
			return Track{}, fmt.Errorf("update track %s: %w", track, err)
		}
	}
	return trackFromAPI(out), nil
}

func trackFromAPI(t *androidpublisher.Track) Track {
	out := Track{Track: t.Track, Releases: []Release{}}
	for _, r := range t.Releases {
		rel := Release{
			Name: r.Name, Status: r.Status, UserFraction: r.UserFraction,
			VersionCodes: []string{}, ReleaseNotes: []ReleaseNote{},
		}
		for _, v := range r.VersionCodes {
			rel.VersionCodes = append(rel.VersionCodes, strconv.FormatInt(v, 10))
		}
		for _, n := range r.ReleaseNotes {
			rel.ReleaseNotes = append(rel.ReleaseNotes, ReleaseNote{Locale: locale.FromPlay(n.Language), Text: n.Text})
		}
		out.Releases = append(out.Releases, rel)
	}
	return out
}
