package apple

import (
	"context"
	"fmt"
	"slices"

	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/metadata"
)

// CodeNotYetReleased is the clierr.Error code of the error for What's New
// before the first release; submit skips release notes on it.
const CodeNotYetReleased = "NOT_YET_RELEASED"

// releaseNotesFile is the metadata file of the What's New text.
const releaseNotesFile = "release_notes.txt"

// ReleaseNote is the What's New text of one locale.
type ReleaseNote struct {
	Locale   string `json:"locale"`
	WhatsNew string `json:"whatsNew"`
}

// SetReleaseNotes sets What's New of v per locale. Every locale must have a
// localization on v. An app without a released version is a *clierr.Error
// with code NOT_YET_RELEASED.
func (s *Service) SetReleaseNotes(ctx context.Context, app App, v Version, notes map[string]string) ([]ReleaseNote, error) {
	return s.setReleaseNotes(ctx, app, v, notes, false)
}

// PreviewReleaseNotes checks like SetReleaseNotes but sends no change.
func (s *Service) PreviewReleaseNotes(ctx context.Context, app App, v Version, notes map[string]string) ([]ReleaseNote, error) {
	return s.setReleaseNotes(ctx, app, v, notes, true)
}

func (s *Service) setReleaseNotes(ctx context.Context, app App, v Version, notes map[string]string, dryRun bool) ([]ReleaseNote, error) {
	field, ok := metadata.FieldByFile(metadata.AppleFields, releaseNotesFile)
	if !ok {
		return nil, clierr.New("internal error: no release notes field")
	}
	locales := make([]string, 0, len(notes))
	for l, text := range notes {
		if err := metadata.ValidateLocaleTexts(l, metadata.AppleFields, metadata.Texts{field.API: text}); err != nil {
			return nil, err
		}
		locales = append(locales, l)
	}
	slices.Sort(locales)
	released, err := s.HasReleasedVersion(ctx, app, v.Platform)
	if err != nil {
		return nil, err
	}
	if !released {
		e := clierr.New("App Store Connect accepts What's New only after the first release")
		e.Code = CodeNotYetReleased
		return nil, e
	}
	ids, err := s.versionLocalizationIDs(ctx, v.ID)
	if err != nil {
		return nil, err
	}
	for _, l := range locales {
		if _, ok := ids[l]; !ok {
			e := clierr.New("version %s has no localization for %s", v.Version, l)
			e.Hint = "store apple listing locale add " + l
			return nil, e
		}
	}
	out := make([]ReleaseNote, 0, len(locales))
	for _, l := range locales {
		if !dryRun {
			attrs := asc.AppStoreVersionLocalizationUpdateRequestDataAttributes{}
			*versionFields[field.API].update(&attrs) = nullable(notes[l])
			id := ids[l]
			_, err := s.client.AppStoreVersionLocalizationsUpdateInstance(ctx, asc.AppStoreVersionLocalizationsUpdateInstanceParams{ID: id},
				asc.AppStoreVersionLocalizationUpdateRequest{Data: asc.AppStoreVersionLocalizationUpdateRequestData{Type: asc.ResourceTypeAppStoreVersionLocalizations, ID: id, Attributes: &attrs}})
			if err != nil {
				return out, fmt.Errorf("set release notes of %s: %w", l, err)
			}
		}
		out = append(out, ReleaseNote{Locale: l, WhatsNew: notes[l]})
	}
	return out, nil
}
