package google

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"slices"
	"strings"

	"google.golang.org/api/androidpublisher/v3"
	"google.golang.org/api/googleapi"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/metadata"
	"github.com/nikeee/store-cli/internal/output"
)

// Listing is the store listing of one language. Locale is the Play language
// code.
type Listing struct {
	Locale           string `json:"locale"`
	Title            string `json:"title"`
	ShortDescription string `json:"shortDescription"`
	FullDescription  string `json:"fullDescription"`
	Video            string `json:"video"`
}

// Listings is the result of ShowListings.
type Listings []Listing

// listingField selects one text in the generated API listing and in Listing.
type listingField struct {
	api    func(*androidpublisher.Listing) *string
	result func(*Listing) *string
}

// listingFields maps metadata.Field.API names to the generated Listing field
// and the result field. It is the only place that knows either; read
// (ListingTexts, listingFromAPI), write (ApplyListingChanges) and rendering
// share it.
var listingFields = map[string]listingField{
	"title": {
		func(l *androidpublisher.Listing) *string { return &l.Title },
		func(l *Listing) *string { return &l.Title },
	},
	"shortDescription": {
		func(l *androidpublisher.Listing) *string { return &l.ShortDescription },
		func(l *Listing) *string { return &l.ShortDescription },
	},
	"fullDescription": {
		func(l *androidpublisher.Listing) *string { return &l.FullDescription },
		func(l *Listing) *string { return &l.FullDescription },
	},
	"video": {
		func(l *androidpublisher.Listing) *string { return &l.Video },
		func(l *Listing) *string { return &l.Video },
	},
}

// RenderText implements output.TextRenderer.
func (l Listing) RenderText(w io.Writer, s output.Styles) error {
	var sb strings.Builder
	l.render(&sb, s)
	_, err := io.WriteString(w, sb.String())
	return err
}

// RenderText implements output.TextRenderer: listings separated by a blank
// line.
func (ls Listings) RenderText(w io.Writer, s output.Styles) error {
	var sb strings.Builder
	for i, l := range ls {
		if i > 0 {
			sb.WriteString("\n")
		}
		l.render(&sb, s)
	}
	_, err := io.WriteString(w, sb.String())
	return err
}

// render writes the locale and every text as "key: value"; multi-line values
// continue indented below their key.
func (l Listing) render(sb *strings.Builder, s output.Styles) {
	sb.WriteString(s.Key.Render("locale:") + " " + l.Locale + "\n")
	for _, f := range metadata.GoogleFields {
		v := *listingFields[f.API].result(&l)
		line := s.Key.Render(f.API + ":")
		if strings.Contains(v, "\n") {
			sb.WriteString(line + "\n")
			for _, ln := range strings.Split(v, "\n") {
				sb.WriteString(strings.TrimRight("  "+ln, " ") + "\n")
			}
			continue
		}
		sb.WriteString(strings.TrimRight(line+" "+v, " ") + "\n")
	}
}

func listingFromAPI(a *androidpublisher.Listing) Listing {
	l := Listing{Locale: a.Language}
	for _, f := range metadata.GoogleFields {
		*listingFields[f.API].result(&l) = *listingFields[f.API].api(a)
	}
	return l
}

func listingTexts(a *androidpublisher.Listing) metadata.Texts {
	t := metadata.Texts{}
	for _, f := range metadata.GoogleFields {
		if v := *listingFields[f.API].api(a); v != "" {
			t[f.API] = v
		}
	}
	return t
}

// ListingTexts returns the texts of every listing language of the edit,
// keyed by Play language code and Field.API. Empty fields are absent.
func (s *Service) ListingTexts(ctx context.Context, editID string) (map[string]metadata.Texts, error) {
	l, err := s.api.Edits.Listings.List(s.packageName, editID).Context(ctx).Do()
	if err != nil {
		return nil, fmt.Errorf("list listings: %w", err)
	}
	out := map[string]metadata.Texts{}
	for _, a := range l.Listings {
		out[a.Language] = listingTexts(a)
	}
	return out, nil
}

// getListing returns the listing of language; nil when it does not exist.
func (s *Service) getListing(ctx context.Context, editID, language string) (*androidpublisher.Listing, error) {
	a, err := s.api.Edits.Listings.Get(s.packageName, editID, language).Context(ctx).Do()
	var ge *googleapi.Error
	if errors.As(err, &ge) && ge.Code == http.StatusNotFound {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read listing %s: %w", language, err)
	}
	return a, nil
}

// ShowListings returns the listings sorted by language, only language when it
// is not empty (a missing one is an error).
func (s *Service) ShowListings(ctx context.Context, language string) (Listings, error) {
	out := Listings{}
	err := s.withEdit(ctx, readOnly, func(editID string) error {
		l, err := s.api.Edits.Listings.List(s.packageName, editID).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("list listings: %w", err)
		}
		for _, a := range l.Listings {
			if language == "" || a.Language == language {
				out = append(out, listingFromAPI(a))
			}
		}
		if language != "" && len(out) == 0 {
			return clierr.New("listing language %s not found", language)
		}
		return nil
	})
	slices.SortFunc(out, func(a, b Listing) int { return strings.Compare(a.Locale, b.Locale) })
	return out, err
}

// overlay applies changes to a copy of cur (nil: a new listing of language).
func overlay(cur *androidpublisher.Listing, language string, changes []metadata.Change) *androidpublisher.Listing {
	merged := androidpublisher.Listing{Language: language}
	if cur != nil {
		merged = *cur
		merged.Language = language
	}
	for _, c := range changes {
		*listingFields[c.Field].api(&merged) = c.New
	}
	return &merged
}

// SetListing changes the given texts of one language (Play language code),
// keeping the fields not given. The texts are validated before any request.
// With DryRun nothing is sent and the result is the listing as it would be.
func (s *Service) SetListing(ctx context.Context, language string, texts metadata.Texts, c CommitOptions) (Listing, error) {
	if err := metadata.ValidateLocaleTexts(language, "", metadata.GoogleFields, texts); err != nil {
		return Listing{}, err
	}
	var res Listing
	err := s.withEdit(ctx, c, func(editID string) error {
		cur, err := s.getListing(ctx, editID, language)
		if err != nil {
			return err
		}
		var remote metadata.Texts
		if cur != nil {
			remote = listingTexts(cur)
		} else if !slices.ContainsFunc(metadata.GoogleFields, func(f metadata.Field) bool { return texts[f.API] != "" }) {
			return clierr.New("listing %s does not exist; give at least one non-empty field to create it", language)
		}
		changes := metadata.DiffTexts(metadata.LevelListing, language, metadata.GoogleFields, texts, remote)
		res = listingFromAPI(overlay(cur, language, changes))
		if len(changes) == 0 && cur != nil {
			return errNoChange
		}
		if c.DryRun {
			return nil
		}
		_, err = s.ApplyListingChanges(ctx, editID, changes)
		return err
	})
	if errors.Is(err, errNoChange) {
		return res, nil
	}
	return res, err
}

// ApplyListingChanges sends the listing changes, one PUT per language with
// the current listing overlaid by the changed fields (a PUT replaces the
// whole listing, so untouched fields such as video must be sent again). A
// language that does not exist yet is created. It returns the changes applied
// so far, also on error.
func (s *Service) ApplyListingChanges(ctx context.Context, editID string, changes []metadata.Change) ([]metadata.Change, error) {
	applied := []metadata.Change{}
	var languages []string
	byLanguage := map[string][]metadata.Change{}
	for _, c := range changes {
		if _, ok := listingFields[c.Field]; !ok || c.Scope != string(metadata.LevelListing) {
			return applied, clierr.New("cannot apply %s change of %q to the listing", c.Scope, c.Field)
		}
		if _, ok := byLanguage[c.Locale]; !ok {
			languages = append(languages, c.Locale)
		}
		byLanguage[c.Locale] = append(byLanguage[c.Locale], c)
	}
	for _, language := range languages {
		cur, err := s.getListing(ctx, editID, language)
		if err != nil {
			return applied, err
		}
		merged := overlay(cur, language, byLanguage[language])
		if _, err := s.api.Edits.Listings.Update(s.packageName, editID, language, merged).Context(ctx).Do(); err != nil {
			return applied, fmt.Errorf("update listing %s: %w", language, err)
		}
		applied = append(applied, byLanguage[language]...)
	}
	return applied, nil
}
