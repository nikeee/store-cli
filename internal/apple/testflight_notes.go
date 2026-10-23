package apple

import (
	"context"
	"fmt"
	"unicode/utf8"

	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/apple/ascclient"
	"github.com/nikeee/store-cli/internal/clierr"
)

// maxBuildNotes is the "What to Test" limit in characters (runes).
const maxBuildNotes = 4000

// BuildNotes is the "What to Test" text of a build in one locale. Build is the
// build number.
type BuildNotes struct {
	Build  string `json:"build"`
	Locale string `json:"locale"`
	Text   string `json:"text"`
}

// SetBuildNotes sets the "What to Test" text of b for locale, creating the
// localization when the build has none for that locale. It does not wait for
// the build to finish processing. With dryRun it validates and looks the
// localization up but sends no change.
func (s *Service) SetBuildNotes(ctx context.Context, b Build, locale, text string, dryRun bool) (BuildNotes, error) {
	if err := CheckBuildNotes(text); err != nil {
		return BuildNotes{}, err
	}
	id, err := s.findBuildNotes(ctx, b, locale)
	if err != nil {
		return BuildNotes{}, err
	}
	if dryRun {
		return BuildNotes{Build: b.Build, Locale: locale, Text: text}, nil
	}
	if id != "" {
		_, err = s.client.BetaBuildLocalizationsUpdateInstance(ctx, asc.BetaBuildLocalizationsUpdateInstanceParams{ID: id}, asc.BetaBuildLocalizationUpdateRequest{
			Data: asc.BetaBuildLocalizationUpdateRequestData{
				Type:       asc.ResourceTypeBetaBuildLocalizations,
				ID:         id,
				Attributes: &asc.BetaBuildLocalizationUpdateRequestDataAttributes{WhatsNew: asc.Value(text)},
			},
		})
	} else {
		_, err = s.client.BetaBuildLocalizationsCreateInstance(ctx, asc.BetaBuildLocalizationCreateRequest{
			Data: asc.BetaBuildLocalizationCreateRequestData{
				Type:       asc.ResourceTypeBetaBuildLocalizations,
				Attributes: asc.BetaBuildLocalizationCreateRequestDataAttributes{Locale: locale, WhatsNew: asc.Value(text)},
				Relationships: asc.BetaBuildLocalizationCreateRequestDataRelationships{
					Build: asc.BetaBuildLocalizationCreateRequestDataRelationshipsBuild{
						Data: asc.ResourceIdentifier{Type: asc.ResourceTypeBuilds, ID: b.ID},
					},
				},
			},
		})
	}
	if err != nil {
		return BuildNotes{}, fmt.Errorf("set notes of build %s (%s): %w", b.Build, locale, err)
	}
	return BuildNotes{Build: b.Build, Locale: locale, Text: text}, nil
}

// CheckBuildNotes reports an error when text exceeds the 4000 character limit.
func CheckBuildNotes(text string) error {
	if n := utf8.RuneCountInString(text); n > maxBuildNotes {
		return clierr.New("notes are %d characters long (maximum %d)", n, maxBuildNotes)
	}
	return nil
}

// findBuildNotes returns the ID of b's localization for locale, or "" if none.
func (s *Service) findBuildNotes(ctx context.Context, b Build, locale string) (string, error) {
	first, err := s.client.BuildsBetaBuildLocalizationsGetToManyRelated(ctx, asc.BuildsBetaBuildLocalizationsGetToManyRelatedParams{
		ID:                           b.ID,
		FieldsBetaBuildLocalizations: []string{"locale"},
		Limit:                        200,
	})
	if err != nil {
		return "", fmt.Errorf("list notes of build %s: %w", b.Build, err)
	}
	id := ""
	err = ascclient.Paginate(ctx, s.client, first, func(p *asc.BetaBuildLocalizationsWithoutIncludesResponse) *string {
		for _, l := range p.Data {
			if l.Attributes != nil && l.Attributes.Locale != nil && *l.Attributes.Locale == locale {
				id = l.ID
				return nil
			}
		}
		return p.Links.Next
	})
	if err != nil {
		return "", fmt.Errorf("list notes of build %s: %w", b.Build, err)
	}
	return id, nil
}
