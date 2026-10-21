package apple

import (
	"time"

	"github.com/nikeee/store-cli/internal/apple/asc"
)

// Version is an App Store version. State is App Store Connect's
// appVersionState.
type Version struct {
	ID                  string        `json:"id"`
	Platform            string        `json:"platform"`
	Version             string        `json:"version"`
	State               string        `json:"state"`
	ReleaseType         string        `json:"releaseType"`
	EarliestReleaseDate *time.Time    `json:"earliestReleaseDate"`
	Build               *VersionBuild `json:"build"`
}

// VersionBuild is the build attached to a version; Build is its build number.
type VersionBuild struct {
	ID    string `json:"id"`
	Build string `json:"build"`
}

// versionsFromAPI maps appStoreVersions fetched with include=build.
func versionsFromAPI(data []asc.AppStoreVersion, included []asc.IncludedResource) ([]Version, error) {
	builds := map[string]string{} // build ID -> build number
	for _, inc := range included {
		if inc.Type != asc.ResourceTypeBuilds {
			continue
		}
		var b asc.Build
		if err := inc.Decode(&b); err != nil {
			return nil, err
		}
		if b.Attributes != nil {
			builds[b.ID] = deref(b.Attributes.Version)
		}
	}
	versions := make([]Version, 0, len(data))
	for _, d := range data {
		v := Version{ID: d.ID}
		if a := d.Attributes; a != nil {
			v.Platform = flagPlatform(deref(a.Platform))
			v.Version = deref(a.VersionString)
			v.State = string(deref(a.AppVersionState))
			v.ReleaseType = string(deref(a.ReleaseType))
			v.EarliestReleaseDate = a.EarliestReleaseDate
		}
		if r := d.Relationships; r != nil && r.Build != nil && r.Build.Data != nil {
			v.Build = &VersionBuild{ID: r.Build.Data.ID, Build: builds[r.Build.Data.ID]}
		}
		versions = append(versions, v)
	}
	return versions, nil
}
