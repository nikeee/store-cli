package apple

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"strings"

	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/apple/ascclient"
	"github.com/nikeee/store-cli/internal/clierr"
)

// BetaGroup is a TestFlight beta group.
type BetaGroup struct {
	ID                string `json:"id"`
	Name              string `json:"name"`
	Internal          bool   `json:"internal"`
	PublicLinkEnabled bool   `json:"publicLinkEnabled"`
	PublicLink        string `json:"publicLink"`
}

// Tester is a TestFlight beta tester. InviteType and State are lowercase.
type Tester struct {
	ID         string `json:"id"`
	Email      string `json:"email"`
	FirstName  string `json:"firstName"`
	LastName   string `json:"lastName"`
	InviteType string `json:"inviteType"`
	State      string `json:"state"`
}

// GroupSpec describes a beta group to create. PublicLinkLimit 0 means no limit.
type GroupSpec struct {
	Name            string
	Internal        bool
	PublicLink      bool
	PublicLinkLimit int
}

// GroupBuild is the result of adding a build to a group (Build is the build number).
type GroupBuild struct {
	Group string `json:"group"`
	Build string `json:"build"`
}

// BetaReview is the Beta App Review state of a build (Build is the build number).
type BetaReview struct {
	Build string `json:"build"`
	State string `json:"state"`
}

func groupFromAPI(g asc.BetaGroup) BetaGroup {
	out := BetaGroup{ID: g.ID}
	if a := g.Attributes; a != nil {
		out.Name = deref(a.Name)
		out.Internal = deref(a.IsInternalGroup)
		out.PublicLinkEnabled = deref(a.PublicLinkEnabled)
		out.PublicLink = deref(a.PublicLink)
	}
	return out
}

func testerFromAPI(t asc.BetaTester) Tester {
	out := Tester{ID: t.ID}
	if a := t.Attributes; a != nil {
		out.Email = deref(a.Email)
		out.FirstName = deref(a.FirstName)
		out.LastName = deref(a.LastName)
		out.InviteType = strings.ToLower(string(deref(a.InviteType)))
		out.State = strings.ToLower(string(deref(a.State)))
	}
	return out
}

// ListGroups returns the beta groups of app.
func (s *Service) ListGroups(ctx context.Context, app App) ([]BetaGroup, error) {
	first, err := s.client.AppsBetaGroupsGetToManyRelated(ctx, asc.AppsBetaGroupsGetToManyRelatedParams{ID: app.ID, Limit: 200})
	if err != nil {
		return nil, fmt.Errorf("list beta groups: %w", err)
	}
	groups := []BetaGroup{}
	err = ascclient.Paginate(ctx, s.client, first, func(p *asc.BetaGroupsWithoutIncludesResponse) *string {
		for _, g := range p.Data {
			groups = append(groups, groupFromAPI(g))
		}
		return p.Links.Next
	})
	if err != nil {
		return nil, fmt.Errorf("list beta groups: %w", err)
	}
	return groups, nil
}

// ResolveGroup returns the group of app whose ID or exact name is nameOrID.
func (s *Service) ResolveGroup(ctx context.Context, app App, nameOrID string) (BetaGroup, error) {
	groups, err := s.ListGroups(ctx, app)
	if err != nil {
		return BetaGroup{}, err
	}
	for _, g := range groups {
		if g.ID == nameOrID {
			return g, nil
		}
	}
	for _, g := range groups {
		if g.Name == nameOrID {
			return g, nil
		}
	}
	names := make([]string, len(groups))
	for i, g := range groups {
		names[i] = g.Name
	}
	existing := "(none)"
	if len(names) > 0 {
		existing = strings.Join(names, ", ")
	}
	return BetaGroup{}, clierr.New("no beta group %q; existing groups: %s", nameOrID, existing)
}

// CreateGroup creates a beta group. With dryRun it sends nothing and returns
// the group as requested (without an ID or public link).
func (s *Service) CreateGroup(ctx context.Context, app App, spec GroupSpec, dryRun bool) (BetaGroup, error) {
	if dryRun {
		return BetaGroup{Name: spec.Name, Internal: spec.Internal, PublicLinkEnabled: spec.PublicLink}, nil
	}
	attrs := asc.BetaGroupCreateRequestDataAttributes{
		Name:              spec.Name,
		IsInternalGroup:   asc.Value(spec.Internal),
		PublicLinkEnabled: asc.Value(spec.PublicLink),
	}
	if spec.PublicLinkLimit > 0 {
		attrs.PublicLinkLimitEnabled = asc.Value(true)
		attrs.PublicLinkLimit = asc.Value(spec.PublicLinkLimit)
	}
	resp, err := s.client.BetaGroupsCreateInstance(ctx, asc.BetaGroupCreateRequest{
		Data: asc.BetaGroupCreateRequestData{
			Type:       asc.ResourceTypeBetaGroups,
			Attributes: attrs,
			Relationships: asc.BetaGroupCreateRequestDataRelationships{
				App: asc.BetaGroupCreateRequestDataRelationshipsApp{Data: asc.ResourceIdentifier{Type: asc.ResourceTypeApps, ID: app.ID}},
			},
		},
	})
	if err != nil {
		return BetaGroup{}, fmt.Errorf("create beta group %q: %w", spec.Name, err)
	}
	return groupFromAPI(resp.Data), nil
}

// AddBuildToGroup adds b to g. With dryRun it sends nothing.
func (s *Service) AddBuildToGroup(ctx context.Context, g BetaGroup, b Build, dryRun bool) (GroupBuild, error) {
	res := GroupBuild{Group: g.Name, Build: b.Build}
	if dryRun {
		return res, nil
	}
	err := s.client.BetaGroupsBuildsCreateToManyRelationship(ctx,
		asc.BetaGroupsBuildsCreateToManyRelationshipParams{ID: g.ID},
		asc.BetaGroupBuildsLinkagesRequest{Data: []asc.ResourceIdentifier{{Type: asc.ResourceTypeBuilds, ID: b.ID}}})
	if err != nil {
		return GroupBuild{}, fmt.Errorf("add build %s to group %q: %w", b.Build, g.Name, err)
	}
	return res, nil
}

// ListTesters returns the testers of app, or of the group with ID groupID if set.
func (s *Service) ListTesters(ctx context.Context, app App, groupID string) ([]Tester, error) {
	p := asc.BetaTestersGetCollectionParams{FilterApps: []string{app.ID}, Limit: 200}
	if groupID != "" {
		p.FilterBetaGroups = []string{groupID}
	}
	return s.collectTesters(ctx, p)
}

func (s *Service) collectTesters(ctx context.Context, params asc.BetaTestersGetCollectionParams) ([]Tester, error) {
	first, err := s.client.BetaTestersGetCollection(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("list testers: %w", err)
	}
	testers := []Tester{}
	err = ascclient.Paginate(ctx, s.client, first, func(p *asc.BetaTestersResponse) *string {
		for _, t := range p.Data {
			testers = append(testers, testerFromAPI(t))
		}
		return p.Links.Next
	})
	if err != nil {
		return nil, fmt.Errorf("list testers: %w", err)
	}
	return testers, nil
}

// findTester returns the tester with email, inside group g if g is non-empty.
func (s *Service) findTester(ctx context.Context, email, groupID string) (Tester, bool, error) {
	p := asc.BetaTestersGetCollectionParams{FilterEmail: []string{email}, Limit: 200}
	if groupID != "" {
		p.FilterBetaGroups = []string{groupID}
	}
	testers, err := s.collectTesters(ctx, p)
	if err != nil || len(testers) == 0 {
		return Tester{}, false, err
	}
	return testers[0], true, nil
}

// AddTester adds the tester with email to g, creating the tester if needed.
// A tester already in the group is left alone. With dryRun it looks the
// tester up but sends no change.
func (s *Service) AddTester(ctx context.Context, g BetaGroup, email, firstName, lastName string, dryRun bool) (Tester, error) {
	if t, ok, err := s.findTester(ctx, email, g.ID); err != nil || ok {
		return t, err
	}
	t, exists, err := s.findTester(ctx, email, "")
	if err != nil {
		return Tester{}, err
	}
	if dryRun {
		if !exists {
			t = Tester{Email: email, FirstName: firstName, LastName: lastName}
		}
		return t, nil
	}
	if exists {
		err = s.client.BetaGroupsBetaTestersCreateToManyRelationship(ctx,
			asc.BetaGroupsBetaTestersCreateToManyRelationshipParams{ID: g.ID},
			asc.BetaGroupBetaTestersLinkagesRequest{Data: []asc.ResourceIdentifier{{Type: asc.ResourceTypeBetaTesters, ID: t.ID}}})
		if err != nil {
			return Tester{}, fmt.Errorf("add %s to group %q: %w", email, g.Name, err)
		}
		return t, nil
	}
	attrs := asc.BetaTesterCreateRequestDataAttributes{Email: email}
	if firstName != "" {
		attrs.FirstName = asc.Value(firstName)
	}
	if lastName != "" {
		attrs.LastName = asc.Value(lastName)
	}
	resp, err := s.client.BetaTestersCreateInstance(ctx, asc.BetaTesterCreateRequest{
		Data: asc.BetaTesterCreateRequestData{
			Type:       asc.ResourceTypeBetaTesters,
			Attributes: attrs,
			Relationships: &asc.BetaTesterCreateRequestDataRelationships{
				BetaGroups: &asc.BetaTesterCreateRequestDataRelationshipsBetaGroups{
					Data: []asc.ResourceIdentifier{{Type: asc.ResourceTypeBetaGroups, ID: g.ID}},
				},
			},
		},
	})
	if err != nil {
		return Tester{}, fmt.Errorf("create tester %s: %w", email, err)
	}
	return testerFromAPI(resp.Data), nil
}

// RemoveTester removes the tester with email from g (the tester itself stays).
// With dryRun it looks the tester up but sends no change.
func (s *Service) RemoveTester(ctx context.Context, g BetaGroup, email string, dryRun bool) (Tester, error) {
	t, ok, err := s.findTester(ctx, email, g.ID)
	if err != nil {
		return Tester{}, err
	}
	if !ok {
		return Tester{}, clierr.New("tester %s is not in group %q", email, g.Name)
	}
	if dryRun {
		return t, nil
	}
	err = s.client.BetaGroupsBetaTestersDeleteToManyRelationship(ctx,
		asc.BetaGroupsBetaTestersDeleteToManyRelationshipParams{ID: g.ID},
		asc.BetaGroupBetaTestersLinkagesRequest{Data: []asc.ResourceIdentifier{{Type: asc.ResourceTypeBetaTesters, ID: t.ID}}})
	if err != nil {
		return Tester{}, fmt.Errorf("remove %s from group %q: %w", email, g.Name, err)
	}
	return t, nil
}

// stateSubmitted is reported when a build is known to be submitted but its
// Beta App Review state could not be read.
const stateSubmitted = "submitted"

// SubmitBetaReview submits b to Beta App Review. A build that is already
// submitted is a success reporting its current state. With dryRun it reads
// the current submission and sends nothing; a build without one reports
// waiting_for_review, the state a submission would start in.
func (s *Service) SubmitBetaReview(ctx context.Context, b Build, dryRun bool) (BetaReview, error) {
	if dryRun {
		state, found, err := s.betaReviewState(ctx, b)
		if err != nil {
			return BetaReview{}, err
		}
		if !found {
			state = strings.ToLower(string(asc.BetaReviewStateWaitingForReview))
		}
		return BetaReview{Build: b.Build, State: state}, nil
	}
	resp, err := s.client.BetaAppReviewSubmissionsCreateInstance(ctx, asc.BetaAppReviewSubmissionCreateRequest{
		Data: asc.BetaAppReviewSubmissionCreateRequestData{
			Type: asc.ResourceTypeBetaAppReviewSubmissions,
			Relationships: asc.BetaAppReviewSubmissionCreateRequestDataRelationships{
				Build: asc.BetaAppReviewSubmissionCreateRequestDataRelationshipsBuild{
					Data: asc.ResourceIdentifier{Type: asc.ResourceTypeBuilds, ID: b.ID},
				},
			},
		},
	})
	var re *asc.ResponseError
	switch {
	case err == nil:
		state := stateSubmitted
		if a := resp.Data.Attributes; a != nil && a.BetaReviewState != nil {
			state = strings.ToLower(string(*a.BetaReviewState))
		}
		return BetaReview{Build: b.Build, State: state}, nil
	case errors.As(err, &re) && re.StatusCode == http.StatusConflict:
		// The spec documents no error code for a 409 on this POST, so a 409
		// counts as "already submitted" only when the build has a submission;
		// otherwise Apple's error stands.
		state, found, lerr := s.betaReviewState(ctx, b)
		if lerr != nil {
			return BetaReview{}, lerr
		}
		if found {
			return BetaReview{Build: b.Build, State: state}, nil
		}
	}
	return BetaReview{}, fmt.Errorf("submit build %s to Beta App Review: %w", b.Build, err)
}

// betaReviewState returns the lowercase Beta App Review state of b, found
// false if b has no submission.
func (s *Service) betaReviewState(ctx context.Context, b Build) (state string, found bool, err error) {
	resp, err := s.client.BuildsBetaAppReviewSubmissionGetToOneRelated(ctx, asc.BuildsBetaAppReviewSubmissionGetToOneRelatedParams{ID: b.ID})
	if err != nil {
		var re *asc.ResponseError
		if errors.As(err, &re) && re.StatusCode == http.StatusNotFound {
			return "", false, nil
		}
		return "", false, fmt.Errorf("read Beta App Review state of build %s: %w", b.Build, err)
	}
	if resp.Data.ID == "" {
		return "", false, nil
	}
	if a := resp.Data.Attributes; a != nil && a.BetaReviewState != nil {
		return strings.ToLower(string(*a.BetaReviewState)), true, nil
	}
	return stateSubmitted, true, nil
}
