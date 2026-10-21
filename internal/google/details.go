package google

import (
	"context"
	"fmt"
	"io"
	"strings"

	"google.golang.org/api/androidpublisher/v3"

	"github.com/nikeee/store-cli/internal/locale"
	"github.com/nikeee/store-cli/internal/output"
)

// Details are the app's contact details and default language.
type Details struct {
	Email         string `json:"email"`
	Website       string `json:"website"`
	Phone         string `json:"phone"`
	DefaultLocale string `json:"defaultLocale"`
}

// DetailsPatch holds the fields to change; nil fields are not sent. An empty
// string clears a field. DefaultLocale is an App Store style locale.
type DetailsPatch struct {
	Email, Website, Phone, DefaultLocale *string
}

// RenderText implements output.TextRenderer.
func (d Details) RenderText(w io.Writer, s output.Styles) error {
	var sb strings.Builder
	for _, f := range [][2]string{{"email", d.Email}, {"website", d.Website}, {"phone", d.Phone}, {"defaultLocale", d.DefaultLocale}} {
		sb.WriteString(strings.TrimRight(s.Key.Render(f[0]+":")+" "+f[1], " ") + "\n")
	}
	_, err := io.WriteString(w, sb.String())
	return err
}

func detailsFromAPI(a *androidpublisher.AppDetails) Details {
	return Details{
		Email: a.ContactEmail, Website: a.ContactWebsite, Phone: a.ContactPhone,
		DefaultLocale: locale.FromPlay(a.DefaultLanguage),
	}
}

// ShowDetails returns the app's contact details.
func (s *Service) ShowDetails(ctx context.Context) (Details, error) {
	var out Details
	err := s.withEdit(ctx, readOnly, func(editID string) error {
		a, err := s.api.Edits.Details.Get(s.packageName, editID).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("read app details: %w", err)
		}
		out = detailsFromAPI(a)
		return nil
	})
	return out, err
}

// SetDetails patches the given fields only. With DryRun nothing is sent and
// the result is the current details with the patch applied.
func (s *Service) SetDetails(ctx context.Context, p DetailsPatch, c CommitOptions) (Details, error) {
	patch := &androidpublisher.AppDetails{}
	set := func(dst *string, name string, v *string) {
		if v != nil {
			*dst = *v
			patch.ForceSendFields = append(patch.ForceSendFields, name)
		}
	}
	set(&patch.ContactEmail, "ContactEmail", p.Email)
	set(&patch.ContactWebsite, "ContactWebsite", p.Website)
	set(&patch.ContactPhone, "ContactPhone", p.Phone)
	if p.DefaultLocale != nil {
		v := locale.ToPlay(*p.DefaultLocale)
		set(&patch.DefaultLanguage, "DefaultLanguage", &v)
	}
	var out Details
	err := s.withEdit(ctx, c, func(editID string) error {
		var a *androidpublisher.AppDetails
		var err error
		if c.DryRun {
			if a, err = s.api.Edits.Details.Get(s.packageName, editID).Context(ctx).Do(); err != nil {
				return fmt.Errorf("read app details: %w", err)
			}
			if len(patch.ForceSendFields) > 0 {
				merged := *a
				mergeDetails(&merged, patch)
				a = &merged
			}
		} else if a, err = s.api.Edits.Details.Patch(s.packageName, editID, patch).Context(ctx).Do(); err != nil {
			return fmt.Errorf("update app details: %w", err)
		}
		out = detailsFromAPI(a)
		return nil
	})
	return out, err
}

func mergeDetails(dst, patch *androidpublisher.AppDetails) {
	for _, f := range patch.ForceSendFields {
		switch f {
		case "ContactEmail":
			dst.ContactEmail = patch.ContactEmail
		case "ContactWebsite":
			dst.ContactWebsite = patch.ContactWebsite
		case "ContactPhone":
			dst.ContactPhone = patch.ContactPhone
		case "DefaultLanguage":
			dst.DefaultLanguage = patch.DefaultLanguage
		}
	}
}
