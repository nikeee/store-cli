package apple

import (
	"context"
	"fmt"
	"slices"
	"strings"
	"time"

	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/apple/ascclient"
	"github.com/nikeee/store-cli/internal/clierr"
)

// EditableVersionStates are the App Store version states in which metadata
// and the build can still be changed.
var EditableVersionStates = []string{"PREPARE_FOR_SUBMISSION", "DEVELOPER_REJECTED", "REJECTED", "METADATA_REJECTED", "INVALID_BINARY"}

// releasedVersionStates are the states of versions that were live once.
var releasedVersionStates = []string{"READY_FOR_DISTRIBUTION", "REPLACED_WITH_NEW_VERSION"}

const stateLive = "READY_FOR_DISTRIBUTION"

// maxVersions is the most versions of one platform a lookup looks at.
const maxVersions = 200

// versionPage is the API's page maximum.
const versionPage = 200

// releaseTypes maps the --release values to App Store Connect release types.
var releaseTypes = map[string]string{
	"manual":         "MANUAL",
	"after-approval": "AFTER_APPROVAL",
	"scheduled":      "SCHEDULED",
}

// ParseReleaseType maps a --release value to the App Store Connect release
// type.
func ParseReleaseType(flag string) (string, error) {
	if t, ok := releaseTypes[flag]; ok {
		return t, nil
	}
	return "", clierr.Usagef("invalid --release %q (use manual, after-approval or scheduled)", flag)
}

// CheckReleaseDate reports a release date that does not fit the release type
// (see ParseReleaseType): SCHEDULED needs one, the other types take none.
func CheckReleaseDate(releaseType string, date *time.Time) error {
	if releaseType == "SCHEDULED" && date == nil {
		return clierr.Usagef("--release scheduled needs --release-date")
	}
	if releaseType != "SCHEDULED" && date != nil {
		return clierr.Usagef("--release-date needs --release scheduled")
	}
	return nil
}

func isEditable(v Version) bool { return slices.Contains(EditableVersionStates, v.State) }

// listVersions returns up to limit versions of app on platform (a --platform
// value), newest first, optionally only those with version string
// versionString.
func (s *Service) listVersions(ctx context.Context, app App, platform, versionString string, limit int) ([]Version, error) {
	p, ok := APIPlatform(platform)
	if !ok {
		return nil, clierr.Usagef("invalid --platform %q", platform)
	}
	params := asc.AppsAppStoreVersionsGetToManyRelatedParams{
		ID:             app.ID,
		FilterPlatform: []string{string(p)},
		Include:        []string{"build"},
		FieldsBuilds:   []string{"version"},
		Limit:          versionPage,
	}
	if versionString != "" {
		params.FilterVersionString = []string{versionString}
	}
	resp, err := s.client.AppsAppStoreVersionsGetToManyRelated(ctx, params)
	if err != nil {
		return nil, err
	}
	// The API has no sort for versions: read all, newest first by creation.
	data, included := resp.Data, resp.Included
	err = ascclient.Paginate(ctx, s.client, resp, func(r *asc.AppStoreVersionsResponse) *string {
		if r != resp {
			data, included = append(data, r.Data...), append(included, r.Included...)
		}
		return r.Links.Next
	})
	if err != nil {
		return nil, err
	}
	slices.SortStableFunc(data, func(a, b asc.AppStoreVersion) int {
		return createdDate(b).Compare(createdDate(a))
	})
	vs, err := versionsFromAPI(data, included)
	if err != nil || len(vs) <= limit {
		return vs, err
	}
	return vs[:limit], nil
}

// createdDate is the creation time of v, the zero time if unknown.
func createdDate(v asc.AppStoreVersion) time.Time {
	if v.Attributes == nil || v.Attributes.CreatedDate == nil {
		return time.Time{}
	}
	return *v.Attributes.CreatedDate
}

// ListVersions returns the newest limit versions of app on platform.
func (s *Service) ListVersions(ctx context.Context, app App, platform string, limit int) ([]Version, error) {
	return s.listVersions(ctx, app, platform, "", limit)
}

// FindVersion returns the version with the given version string on platform;
// found is false when it does not exist.
func (s *Service) FindVersion(ctx context.Context, app App, version, platform string) (Version, bool, error) {
	vs, err := s.listVersions(ctx, app, platform, version, maxVersions)
	if err != nil {
		return Version{}, false, err
	}
	for _, v := range vs {
		if v.Version == version {
			return v, true, nil
		}
	}
	return Version{}, false, nil
}

// noEditableVersion is the error for versions vs (newest first) without an
// editable one. A version newer than the live one (e.g. in review) is named,
// since creating another version does not help then.
func noEditableVersion(vs []Version) *clierr.Error {
	if len(vs) > 0 && !slices.Contains(releasedVersionStates, vs[0].State) {
		return withCancelHint(stateError("no editable version: %s is in state %s", vs[0].Version, vs[0].State), vs[0])
	}
	e := clierr.New("no editable version")
	e.Hint = "store apple version create <version>"
	return e
}

// withCancelHint points e at version cancel when v waits for or is in review.
func withCancelHint(e *clierr.Error, v Version) *clierr.Error {
	if v.State == "WAITING_FOR_REVIEW" || v.State == "IN_REVIEW" {
		e.Hint = "store apple version cancel " + v.Version
	}
	return e
}

// editableVersions returns the editable versions of vs.
func editableVersions(vs []Version) []Version {
	return slices.DeleteFunc(slices.Clone(vs), func(v Version) bool { return !isEditable(v) })
}

// singleEditable returns the one editable version of vs (newest first).
func singleEditable(vs []Version) (Version, error) {
	es := editableVersions(vs)
	switch len(es) {
	case 0:
		return Version{}, noEditableVersion(vs)
	case 1:
		return es[0], nil
	}
	names := make([]string, len(es))
	for i, v := range es {
		names[i] = fmt.Sprintf("%s (%s)", v.Version, v.State)
	}
	return Version{}, clierr.New("more than one editable version: %s", strings.Join(names, ", "))
}

// EditableVersion returns the one version of app on platform that can still
// be edited. None or several are errors.
func (s *Service) EditableVersion(ctx context.Context, app App, platform string) (Version, error) {
	vs, err := s.listVersions(ctx, app, platform, "", maxVersions)
	if err != nil {
		return Version{}, err
	}
	return singleEditable(vs)
}

// CurrentVersion returns the editable version of app on platform, else the
// one that is live.
func (s *Service) CurrentVersion(ctx context.Context, app App, platform string) (Version, error) {
	vs, err := s.listVersions(ctx, app, platform, "", maxVersions)
	if err != nil {
		return Version{}, err
	}
	if slices.ContainsFunc(vs, isEditable) {
		return singleEditable(vs)
	}
	for _, v := range vs {
		if v.State == stateLive {
			return v, nil
		}
	}
	return Version{}, noEditableVersion(vs)
}

// HasReleasedVersion reports whether app has, on platform, a version that is
// or was live.
func (s *Service) HasReleasedVersion(ctx context.Context, app App, platform string) (bool, error) {
	vs, err := s.listVersions(ctx, app, platform, "", maxVersions)
	if err != nil {
		return false, err
	}
	return slices.ContainsFunc(vs, func(v Version) bool { return slices.Contains(releasedVersionStates, v.State) }), nil
}

// CreateVersionOptions are the inputs of CreateVersion. ReleaseType is an
// App Store Connect release type (see ParseReleaseType), empty for the API
// default. A nil Copyright or ReleaseDate is left unset. DryRun checks
// everything but creates nothing.
type CreateVersionOptions struct {
	Version     string
	Platform    string
	ReleaseType string
	ReleaseDate *time.Time
	Copyright   *string
	DryRun      bool
}

// CreateVersion creates the version o.Version; an existing one is an error.
func (s *Service) CreateVersion(ctx context.Context, app App, o CreateVersionOptions) (Version, error) {
	p, ok := APIPlatform(o.Platform)
	if !ok {
		return Version{}, clierr.Usagef("invalid --platform %q", o.Platform)
	}
	if err := CheckReleaseDate(o.ReleaseType, o.ReleaseDate); err != nil {
		return Version{}, err
	}
	if existing, found, err := s.FindVersion(ctx, app, o.Version, o.Platform); err != nil {
		return Version{}, err
	} else if found {
		return Version{}, clierr.New("version %s already exists (%s)", o.Version, existing.State)
	}
	v := Version{Platform: o.Platform, Version: o.Version, State: "PREPARE_FOR_SUBMISSION", ReleaseType: o.ReleaseType, EarliestReleaseDate: o.ReleaseDate}
	if o.DryRun {
		return v, nil
	}
	attrs := asc.AppStoreVersionCreateRequestDataAttributes{Platform: p, VersionString: o.Version}
	if o.ReleaseType != "" {
		attrs.ReleaseType = asc.Value(asc.AppStoreVersionCreateRequestDataAttributesReleaseType(o.ReleaseType))
	}
	if o.ReleaseDate != nil {
		attrs.EarliestReleaseDate = asc.Value(*o.ReleaseDate)
	}
	if o.Copyright != nil {
		attrs.Copyright = asc.Value(*o.Copyright)
	}
	resp, err := s.client.AppStoreVersionsCreateInstance(ctx, asc.AppStoreVersionCreateRequest{Data: asc.AppStoreVersionCreateRequestData{
		Type:       asc.ResourceTypeAppStoreVersions,
		Attributes: attrs,
		Relationships: asc.AppStoreVersionCreateRequestDataRelationships{
			App: asc.AppStoreVersionCreateRequestDataRelationshipsApp{Data: asc.ResourceIdentifier{Type: asc.ResourceTypeApps, ID: app.ID}},
		},
	}})
	if err != nil {
		return Version{}, fmt.Errorf("create version %s: %w", o.Version, err)
	}
	vs, err := versionsFromAPI([]asc.AppStoreVersion{resp.Data}, resp.Included)
	if err != nil {
		return Version{}, err
	}
	return vs[0], nil
}

// CheckSetVersionBuild reports why b cannot be attached to v: only valid
// builds go to editable versions.
func CheckSetVersionBuild(v Version, b Build) error {
	if !isEditable(v) {
		return clierr.New("version %s is in state %s and cannot be edited", v.Version, v.State)
	}
	if b.State != "valid" {
		return clierr.New("build %s is %s, only valid builds can be attached", b.Build, b.State)
	}
	return nil
}

// SetVersionBuild attaches b to v. The version must be editable and the build
// valid.
func (s *Service) SetVersionBuild(ctx context.Context, v Version, b Build) (Version, error) {
	if err := CheckSetVersionBuild(v, b); err != nil {
		return Version{}, err
	}
	err := s.client.AppStoreVersionsBuildUpdateToOneRelationship(ctx,
		asc.AppStoreVersionsBuildUpdateToOneRelationshipParams{ID: v.ID},
		asc.AppStoreVersionBuildLinkageRequest{Data: asc.ResourceIdentifier{Type: asc.ResourceTypeBuilds, ID: b.ID}})
	if err != nil {
		return Version{}, fmt.Errorf("attach build %s to version %s: %w", b.Build, v.Version, err)
	}
	v.Build = &VersionBuild{ID: b.ID, Build: b.Build}
	return v, nil
}
