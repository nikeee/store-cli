package google

import (
	"context"
	"fmt"
	"io"
	"strings"

	"google.golang.org/api/androidpublisher/v3"

	"github.com/nikeee/store-cli/internal/output"
)

// Testers are the Google Groups that test a track.
type Testers struct {
	Track        string   `json:"track"`
	GoogleGroups []string `json:"googleGroups"`
}

// RenderText implements output.TextRenderer.
func (t Testers) RenderText(w io.Writer, s output.Styles) error {
	var sb strings.Builder
	sb.WriteString(s.Key.Render("track:") + " " + t.Track + "\n")
	if len(t.GoogleGroups) == 0 {
		sb.WriteString("  " + s.Dim.Render("no tester groups") + "\n")
	} else {
		sb.WriteString("  " + s.Key.Render("googleGroups:") + "\n")
		for _, g := range t.GoogleGroups {
			sb.WriteString("    " + g + "\n")
		}
	}
	_, err := io.WriteString(w, sb.String())
	return err
}

func testersFromAPI(track string, t *androidpublisher.Testers) Testers {
	out := Testers{Track: track, GoogleGroups: []string{}}
	if t != nil {
		out.GoogleGroups = append(out.GoogleGroups, t.GoogleGroups...)
	}
	return out
}

// ShowTesters returns the tester groups of a track.
func (s *Service) ShowTesters(ctx context.Context, track string) (Testers, error) {
	var out Testers
	err := s.withEdit(ctx, readOnly, func(editID string) error {
		t, err := s.api.Edits.Testers.Get(s.packageName, editID, track).Context(ctx).Do()
		if err != nil {
			return fmt.Errorf("read testers of track %s: %w", track, err)
		}
		out = testersFromAPI(track, t)
		return nil
	})
	return out, err
}

// SetTesters replaces the tester groups of a track. With DryRun nothing is
// sent: the track is read, so an unknown track is an error, and the result
// is what would be stored.
func (s *Service) SetTesters(ctx context.Context, track string, groups []string, c CommitOptions) (Testers, error) {
	var out Testers
	err := s.withEdit(ctx, c, func(editID string) error {
		t := &androidpublisher.Testers{GoogleGroups: groups}
		var err error
		if c.DryRun {
			if _, err = s.api.Edits.Testers.Get(s.packageName, editID, track).Context(ctx).Do(); err != nil {
				return fmt.Errorf("read testers of track %s: %w", track, err)
			}
		} else if t, err = s.api.Edits.Testers.Update(s.packageName, editID, track, t).Context(ctx).Do(); err != nil {
			return fmt.Errorf("update testers of track %s: %w", track, err)
		}
		out = testersFromAPI(track, t)
		return nil
	})
	return out, err
}
