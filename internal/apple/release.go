package apple

import (
	"context"
	"errors"
	"fmt"
	"net/http"

	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/clierr"
)

const stateForRelease = "PENDING_DEVELOPER_RELEASE"

// PhasedRelease is the phased release of a version. CurrentDayNumber is 0
// while it has not started.
type PhasedRelease struct {
	Version          string `json:"version"`
	State            string `json:"state"`
	CurrentDayNumber int    `json:"currentDayNumber"`
}

func stateError(format string, args ...any) *clierr.Error {
	e := clierr.New(format, args...)
	e.Code = "STATE_ERROR"
	return e
}

// ReleaseVersion releases v, which must wait in PENDING_DEVELOPER_RELEASE.
// The returned version is v as it was found. With dryRun nothing is sent.
func (s *Service) ReleaseVersion(ctx context.Context, v Version, dryRun bool) (Version, error) {
	if v.State != stateForRelease {
		return Version{}, stateError("version %s is in state %s, only %s can be released", v.Version, v.State, stateForRelease)
	}
	if dryRun {
		return v, nil
	}
	_, err := s.client.AppStoreVersionReleaseRequestsCreateInstance(ctx, asc.AppStoreVersionReleaseRequestCreateRequest{Data: asc.AppStoreVersionReleaseRequestCreateRequestData{
		Type: asc.ResourceTypeAppStoreVersionReleaseRequests,
		Relationships: asc.AppStoreVersionReleaseRequestCreateRequestDataRelationships{
			AppStoreVersion: asc.AppStoreVersionReleaseRequestCreateRequestDataRelationshipsAppStoreVersion{Data: asc.ResourceIdentifier{Type: asc.ResourceTypeAppStoreVersions, ID: v.ID}},
		},
	}})
	if err != nil {
		return Version{}, fmt.Errorf("release version %s: %w", v.Version, err)
	}
	return v, nil
}

// phasedOf maps a phased release resource of version v.
func phasedOf(v Version, r asc.AppStoreVersionPhasedRelease) PhasedRelease {
	p := PhasedRelease{Version: v.Version}
	if a := r.Attributes; a != nil {
		p.State = string(deref(a.PhasedReleaseState))
		p.CurrentDayNumber = deref(a.CurrentDayNumber)
	}
	return p
}

// findPhased returns the phased release of v; found is false when v has none.
func (s *Service) findPhased(ctx context.Context, v Version) (asc.AppStoreVersionPhasedRelease, bool, error) {
	resp, err := s.client.AppStoreVersionsAppStoreVersionPhasedReleaseGetToOneRelated(ctx, asc.AppStoreVersionsAppStoreVersionPhasedReleaseGetToOneRelatedParams{ID: v.ID})
	var re *asc.ResponseError
	if errors.As(err, &re) && re.StatusCode == http.StatusNotFound {
		return asc.AppStoreVersionPhasedRelease{}, false, nil
	}
	if err != nil {
		return asc.AppStoreVersionPhasedRelease{}, false, err
	}
	return resp.Data, resp.Data.ID != "", nil
}

// StartPhasedRelease enables phased release for v. An existing phased
// release is returned unchanged. It is created ACTIVE when v is live and
// INACTIVE (it starts with the release) otherwise. With dryRun nothing is
// created and the result shows what would be.
func (s *Service) StartPhasedRelease(ctx context.Context, v Version, dryRun bool) (PhasedRelease, error) {
	existing, found, err := s.findPhased(ctx, v)
	if err != nil {
		return PhasedRelease{}, err
	}
	if found {
		return phasedOf(v, existing), nil
	}
	state := asc.PhasedReleaseStateInactive
	if v.State == stateLive {
		state = asc.PhasedReleaseStateActive
	}
	if dryRun {
		return PhasedRelease{Version: v.Version, State: string(state)}, nil
	}
	resp, err := s.client.AppStoreVersionPhasedReleasesCreateInstance(ctx, asc.AppStoreVersionPhasedReleaseCreateRequest{Data: asc.AppStoreVersionPhasedReleaseCreateRequestData{
		Type:       asc.ResourceTypeAppStoreVersionPhasedReleases,
		Attributes: &asc.AppStoreVersionPhasedReleaseCreateRequestDataAttributes{PhasedReleaseState: asc.Value(state)},
		Relationships: asc.AppStoreVersionPhasedReleaseCreateRequestDataRelationships{
			AppStoreVersion: asc.AppStoreVersionPhasedReleaseCreateRequestDataRelationshipsAppStoreVersion{Data: asc.ResourceIdentifier{Type: asc.ResourceTypeAppStoreVersions, ID: v.ID}},
		},
	}})
	if err != nil {
		return PhasedRelease{}, fmt.Errorf("start phased release of version %s: %w", v.Version, err)
	}
	return phasedOf(v, resp.Data), nil
}

// SetPhasedReleaseState moves the phased release of v to state (PAUSED,
// ACTIVE or COMPLETE). A version without one is an error. With dryRun
// nothing is changed and the result shows the new state.
func (s *Service) SetPhasedReleaseState(ctx context.Context, v Version, state asc.PhasedReleaseState, dryRun bool) (PhasedRelease, error) {
	existing, found, err := s.findPhased(ctx, v)
	if err != nil {
		return PhasedRelease{}, err
	}
	if !found {
		e := stateError("version %s has no phased release", v.Version)
		e.Hint = "store apple version phased start " + v.Version
		return PhasedRelease{}, e
	}
	if dryRun {
		p := phasedOf(v, existing)
		p.State = string(state)
		return p, nil
	}
	resp, err := s.client.AppStoreVersionPhasedReleasesUpdateInstance(ctx,
		asc.AppStoreVersionPhasedReleasesUpdateInstanceParams{ID: existing.ID},
		asc.AppStoreVersionPhasedReleaseUpdateRequest{Data: asc.AppStoreVersionPhasedReleaseUpdateRequestData{
			Type:       asc.ResourceTypeAppStoreVersionPhasedReleases,
			ID:         existing.ID,
			Attributes: &asc.AppStoreVersionPhasedReleaseUpdateRequestDataAttributes{PhasedReleaseState: asc.Value(state)},
		}})
	if err != nil {
		return PhasedRelease{}, fmt.Errorf("set phased release of version %s to %s: %w", v.Version, state, err)
	}
	return phasedOf(v, resp.Data), nil
}
