package apple

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/clierr"
)

// Build is a processed or processing build. Version is the app version
// (pre-release version) it belongs to, Build its build number, Platform the
// --platform value and State the lowercase processing state.
type Build struct {
	ID                      string    `json:"id"`
	Version                 string    `json:"version"`
	Build                   string    `json:"build"`
	Platform                string    `json:"platform"`
	State                   string    `json:"state"`
	UploadedAt              time.Time `json:"uploadedAt"`
	Expired                 bool      `json:"expired"`
	UsesNonExemptEncryption *bool     `json:"usesNonExemptEncryption"`
}

// BuildFilter narrows ListBuilds. State is a --state value (processing,
// failed, invalid or valid), Platform a --platform value; empty fields do
// not filter. Limit 0 means the API default.
type BuildFilter struct {
	Version  string
	State    string
	Limit    int
	Platform string
}

// BuildCondition is what WaitForBuild waits for.
type BuildCondition int

const (
	// BuildExists waits until the build is visible in App Store Connect.
	BuildExists BuildCondition = iota
	// BuildProcessed waits until the build's processing state is valid.
	BuildProcessed
)

// buildStates maps the --state values to the API processing states.
var buildStates = map[string]asc.BuildAttributesProcessingState{
	"processing": asc.BuildAttributesProcessingStateProcessing,
	"failed":     asc.BuildAttributesProcessingStateFailed,
	"invalid":    asc.BuildAttributesProcessingStateInvalid,
	"valid":      asc.BuildAttributesProcessingStateValid,
}

// buildsFromAPI maps builds fetched with include=preReleaseVersion.
func buildsFromAPI(data []asc.Build, included []asc.IncludedResource) ([]Build, error) {
	pre := map[string]asc.PrereleaseVersionAttributes{}
	for _, inc := range included {
		if inc.Type != asc.ResourceTypePreReleaseVersions {
			continue
		}
		var p asc.PrereleaseVersion
		if err := inc.Decode(&p); err != nil {
			return nil, err
		}
		if p.Attributes != nil {
			pre[p.ID] = *p.Attributes
		}
	}
	builds := make([]Build, 0, len(data))
	for _, d := range data {
		b := buildFromAPI(d)
		if r := d.Relationships; r != nil && r.PreReleaseVersion != nil && r.PreReleaseVersion.Data != nil {
			p := pre[r.PreReleaseVersion.Data.ID]
			b.Version = deref(p.Version)
			b.Platform = flagPlatform(deref(p.Platform))
		}
		builds = append(builds, b)
	}
	return builds, nil
}

// buildFromAPI maps the attributes of d; Version and Platform come from the
// pre-release version and are left empty.
func buildFromAPI(d asc.Build) Build {
	b := Build{ID: d.ID}
	if a := d.Attributes; a != nil {
		b.Build = deref(a.Version)
		b.State = strings.ToLower(string(deref(a.ProcessingState)))
		b.UploadedAt = deref(a.UploadedDate)
		b.Expired = deref(a.Expired)
		b.UsesNonExemptEncryption = a.UsesNonExemptEncryption
	}
	return b
}

// ListBuilds returns the builds of app, newest first.
func (s *Service) ListBuilds(ctx context.Context, app App, f BuildFilter) ([]Build, error) {
	p, err := buildQuery(app, f)
	if err != nil {
		return nil, err
	}
	p.Limit = f.Limit
	resp, err := s.client.BuildsGetCollection(ctx, p)
	if err != nil {
		return nil, err
	}
	return buildsFromAPI(resp.Data, resp.Included)
}

func buildQuery(app App, f BuildFilter) (asc.BuildsGetCollectionParams, error) {
	p := asc.BuildsGetCollectionParams{
		FilterApp: []string{app.ID},
		Sort:      []string{"-uploadedDate"},
		Include:   []string{"preReleaseVersion"},
	}
	if f.Version != "" {
		p.FilterPreReleaseVersionVersion = []string{f.Version}
	}
	if f.State != "" {
		state, ok := buildStates[f.State]
		if !ok {
			return p, clierr.Usagef("invalid --state %q (use processing, failed, invalid or valid)", f.State)
		}
		p.FilterProcessingState = []string{string(state)}
	}
	if f.Platform != "" {
		platform, ok := APIPlatform(f.Platform)
		if !ok {
			return p, clierr.Usagef("invalid --platform %q", f.Platform)
		}
		p.FilterPreReleaseVersionPlatform = []string{string(platform)}
	}
	return p, nil
}

// ResolveBuild returns the build ref ("latest" or a build number) of app.
// platform is a --platform value. version narrows it to one app version; a number that exists under several
// versions needs it.
func (s *Service) ResolveBuild(ctx context.Context, app App, ref, version, platform string) (Build, error) {
	b, found, err := s.findBuild(ctx, app, ref, version, platform)
	if err != nil {
		return Build{}, err
	}
	if !found {
		return Build{}, notFound(ref, version)
	}
	return b, nil
}

func notFound(ref, version string) *clierr.Error {
	msg := fmt.Sprintf("build %s not found", ref)
	if version != "" {
		msg += " for version " + version
	}
	e := clierr.New("%s", msg)
	e.Hint = "store apple build list"
	return e
}

// findBuild looks ref up; found is false when no such build exists (yet).
func (s *Service) findBuild(ctx context.Context, app App, ref, version, platform string) (b Build, found bool, err error) {
	if platform == "" {
		return Build{}, false, clierr.New("internal error: no platform given to find build %s", ref)
	}
	p, err := buildQuery(app, BuildFilter{Version: version, Platform: platform})
	if err != nil {
		return Build{}, false, err
	}
	if ref == "latest" {
		p.Limit = 1
	} else {
		p.FilterVersion = []string{ref}
		p.Limit = 200
	}
	resp, err := s.client.BuildsGetCollection(ctx, p)
	if err != nil {
		return Build{}, false, err
	}
	builds, err := buildsFromAPI(resp.Data, resp.Included)
	if err != nil || len(builds) == 0 {
		return Build{}, false, err
	}
	if ref != "latest" {
		var versions []string
		for _, c := range builds {
			if !slices.Contains(versions, c.Version) {
				versions = append(versions, c.Version)
			}
		}
		if len(versions) > 1 {
			slices.Sort(versions)
			return Build{}, false, clierr.Usagef("build %s exists for versions %s; pass --version", ref, strings.Join(versions, ", "))
		}
	}
	return builds[0], true, nil
}

// WaitForBuild polls until build buildNumber (under version, if given, on
// platform, a --platform value)
// satisfies until. A build that ends FAILED or INVALID is an error when
// waiting for BuildProcessed. It gives up after timeout.
func (s *Service) WaitForBuild(ctx context.Context, app App, buildNumber, version, platform string, until BuildCondition, timeout time.Duration) (Build, error) {
	start := s.now()
	for {
		b, found, err := s.findBuild(ctx, app, buildNumber, version, platform)
		if err != nil {
			return Build{}, err
		}
		elapsed := s.now().Sub(start)
		state := "not visible yet"
		if found {
			state = b.State
			switch {
			case until == BuildExists:
				return b, nil
			case b.State == "valid":
				return b, nil
			case b.State == "failed" || b.State == "invalid":
				return Build{}, clierr.New("build %s ended %s in App Store Connect", buildNumber, b.State)
			}
		}
		if elapsed >= timeout {
			what := "to be processed"
			if until == BuildExists {
				what = "to appear"
			}
			return Build{}, clierr.New("timed out after %s waiting for build %s %s", timeout, buildNumber, what)
		}
		s.report.Progressf("waiting for build %s (%s, %s elapsed)", buildNumber, state, shortDuration(elapsed))
		if err := s.poll.Sleep(ctx, s.poll.Interval); err != nil {
			return Build{}, err
		}
	}
}

// shortDuration formats d as whole seconds below a minute, else whole minutes.
func shortDuration(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	return fmt.Sprintf("%dm", int(d.Minutes()))
}

// SetEncryption sets the export compliance answer of b.
func (s *Service) SetEncryption(ctx context.Context, b Build, uses bool) (Build, error) {
	return s.patchBuild(ctx, b, asc.BuildUpdateRequestDataAttributes{UsesNonExemptEncryption: asc.Value(uses)})
}

// ExpireBuild expires b in TestFlight.
func (s *Service) ExpireBuild(ctx context.Context, b Build) (Build, error) {
	return s.patchBuild(ctx, b, asc.BuildUpdateRequestDataAttributes{Expired: asc.Value(true)})
}

// patchBuild updates b and returns it with the attributes the API echoes;
// the response carries no pre-release version, so Version and Platform stay.
func (s *Service) patchBuild(ctx context.Context, b Build, attrs asc.BuildUpdateRequestDataAttributes) (Build, error) {
	resp, err := s.client.BuildsUpdateInstance(ctx, asc.BuildsUpdateInstanceParams{ID: b.ID}, asc.BuildUpdateRequest{
		Data: asc.BuildUpdateRequestData{Type: asc.ResourceTypeBuilds, ID: b.ID, Attributes: &attrs},
	})
	if err != nil {
		return Build{}, fmt.Errorf("update build %s: %w", b.Build, err)
	}
	updated := buildFromAPI(resp.Data)
	updated.Version, updated.Platform = b.Version, b.Platform
	return updated, nil
}
