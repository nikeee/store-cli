package apple

import (
	"context"
	"fmt"
	"slices"

	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/clierr"
)

// stateDryRun is the state reported by the preview of a submission change.
const stateDryRun = "dry-run"

// cancelableSubmissionStates are the review submission states in which a
// submission can be withdrawn.
var cancelableSubmissionStates = []string{"WAITING_FOR_REVIEW", "IN_REVIEW", "UNRESOLVED_ISSUES"}

// SubmissionResult is the outcome of submitting or canceling a version.
// Items is the number of items the review submission holds afterwards.
type SubmissionResult struct {
	Version      string `json:"version"`
	SubmissionID string `json:"submissionId"`
	State        string `json:"state"`
	Items        int    `json:"items"`
}

// reviewSubmission is a review submission with what the lookups need.
type reviewSubmission struct {
	id          string
	state       string
	versionID   string // appStoreVersionForReview, empty if none
	versionName string // its version string when the API included it
	items       int
}

// listReviewSubmissions returns the review submissions of app on platform (a
// --platform value) in one of states.
func (s *Service) listReviewSubmissions(ctx context.Context, app App, platform string, states []string) ([]reviewSubmission, error) {
	p, ok := APIPlatform(platform)
	if !ok {
		return nil, clierr.Usagef("invalid --platform %q", platform)
	}
	resp, err := s.client.AppsReviewSubmissionsGetToManyRelated(ctx, asc.AppsReviewSubmissionsGetToManyRelatedParams{
		ID:             app.ID,
		FilterPlatform: []string{string(p)},
		FilterState:    states,
		Include:        []string{"items", "appStoreVersionForReview"},
	})
	if err != nil {
		return nil, err
	}
	names := map[string]string{} // version ID -> version string
	includedItems := 0
	for _, inc := range resp.Included {
		switch inc.Type {
		case asc.ResourceTypeAppStoreVersions:
			var v asc.AppStoreVersion
			if err := inc.Decode(&v); err != nil {
				return nil, err
			}
			if v.Attributes != nil {
				names[v.ID] = deref(v.Attributes.VersionString)
			}
		case asc.ResourceTypeReviewSubmissionItems:
			includedItems++
		}
	}
	subs := make([]reviewSubmission, 0, len(resp.Data))
	for _, d := range resp.Data {
		sub := reviewSubmission{id: d.ID, items: includedItems}
		if a := d.Attributes; a != nil {
			sub.state = string(deref(a.State))
		}
		if r := d.Relationships; r != nil {
			if r.AppStoreVersionForReview != nil && r.AppStoreVersionForReview.Data != nil {
				sub.versionID = r.AppStoreVersionForReview.Data.ID
				sub.versionName = names[sub.versionID]
			}
			if r.Items != nil && len(r.Items.Data) > 0 {
				sub.items = len(r.Items.Data)
			}
		}
		subs = append(subs, sub)
	}
	return subs, nil
}

// submitPlan is what submitting a version has to do.
type submitPlan struct {
	draft   *reviewSubmission // nil: a submission has to be created
	addItem bool              // the version is not in the draft yet
}

// items is the number of items the submission holds after the plan ran.
func (p submitPlan) items() int {
	n := 0
	if p.draft != nil {
		n = p.draft.items
	}
	if p.addItem {
		n++
	}
	return n
}

// planSubmit checks that v can be submitted and finds the open draft
// submission to reuse.
func (s *Service) planSubmit(ctx context.Context, app App, v Version) (submitPlan, error) {
	if !isEditable(v) {
		e := clierr.New("version %s is in state %s, nothing to submit", v.Version, v.State)
		e.Code = "STATE_ERROR"
		if v.State == "WAITING_FOR_REVIEW" || v.State == "IN_REVIEW" {
			e.Hint = "store apple version cancel " + v.Version
		}
		return submitPlan{}, e
	}
	drafts, err := s.listReviewSubmissions(ctx, app, v.Platform, []string{"READY_FOR_REVIEW"})
	if err != nil {
		return submitPlan{}, err
	}
	if len(drafts) == 0 {
		return submitPlan{addItem: true}, nil
	}
	// Prefer the draft that already holds v, else the first one.
	d := drafts[0]
	if i := slices.IndexFunc(drafts, func(d reviewSubmission) bool { return d.versionID == v.ID }); i >= 0 {
		d = drafts[i]
	}
	if d.versionID != "" && d.versionID != v.ID {
		other := d.versionName
		if other == "" {
			other = d.versionID
		}
		e := clierr.New("the open review submission %s already holds version %s", d.id, other)
		e.Code = "STATE_ERROR"
		return submitPlan{}, e
	}
	return submitPlan{draft: &d, addItem: d.versionID != v.ID}, nil
}

// SubmitVersion submits v for App Review. An open draft review submission of
// the app is reused with its other items; v is added unless the draft holds
// it already.
func (s *Service) SubmitVersion(ctx context.Context, app App, v Version) (SubmissionResult, error) {
	plan, err := s.planSubmit(ctx, app, v)
	if err != nil {
		return SubmissionResult{}, err
	}
	id := ""
	if plan.draft != nil {
		id = plan.draft.id
		s.report.Progressf("reusing the open review submission with %d item(s)", plan.draft.items)
	} else {
		p, _ := APIPlatform(v.Platform)
		resp, err := s.client.ReviewSubmissionsCreateInstance(ctx, asc.ReviewSubmissionCreateRequest{Data: asc.ReviewSubmissionCreateRequestData{
			Type:       asc.ResourceTypeReviewSubmissions,
			Attributes: &asc.ReviewSubmissionCreateRequestDataAttributes{Platform: asc.Value(p)},
			Relationships: asc.ReviewSubmissionCreateRequestDataRelationships{
				App: asc.ReviewSubmissionCreateRequestDataRelationshipsApp{Data: asc.ResourceIdentifier{Type: asc.ResourceTypeApps, ID: app.ID}},
			},
		}})
		if err != nil {
			return SubmissionResult{}, fmt.Errorf("create review submission: %w", err)
		}
		id = resp.Data.ID
	}
	if plan.addItem {
		_, err := s.client.ReviewSubmissionItemsCreateInstance(ctx, asc.ReviewSubmissionItemCreateRequest{Data: asc.ReviewSubmissionItemCreateRequestData{
			Type: asc.ResourceTypeReviewSubmissionItems,
			Relationships: asc.ReviewSubmissionItemCreateRequestDataRelationships{
				ReviewSubmission: asc.ReviewSubmissionItemCreateRequestDataRelationshipsReviewSubmission{Data: asc.ResourceIdentifier{Type: asc.ResourceTypeReviewSubmissions, ID: id}},
				AppStoreVersion:  &asc.ReviewSubmissionItemCreateRequestDataRelationshipsAppStoreVersion{Data: &asc.ResourceIdentifier{Type: asc.ResourceTypeAppStoreVersions, ID: v.ID}},
			},
		}})
		if err != nil {
			return SubmissionResult{}, fmt.Errorf("add version %s to review submission %s: %w", v.Version, id, err)
		}
	}
	state, err := s.updateSubmission(ctx, id, asc.ReviewSubmissionUpdateRequestDataAttributes{Submitted: asc.Value(true)})
	if err != nil {
		return SubmissionResult{}, fmt.Errorf("submit version %s: %w", v.Version, err)
	}
	return SubmissionResult{Version: v.Version, SubmissionID: id, State: state, Items: plan.items()}, nil
}

// PreviewSubmitVersion runs the checks of SubmitVersion and reports what it
// would do, without changing anything. SubmissionID is empty when a new
// submission would be created.
func (s *Service) PreviewSubmitVersion(ctx context.Context, app App, v Version) (SubmissionResult, error) {
	plan, err := s.planSubmit(ctx, app, v)
	if err != nil {
		return SubmissionResult{}, err
	}
	res := SubmissionResult{Version: v.Version, State: stateDryRun, Items: plan.items()}
	if plan.draft != nil {
		res.SubmissionID = plan.draft.id
	}
	return res, nil
}

// findCancelable returns the submission that holds v and can be withdrawn.
func (s *Service) findCancelable(ctx context.Context, app App, v Version) (reviewSubmission, error) {
	subs, err := s.listReviewSubmissions(ctx, app, v.Platform, cancelableSubmissionStates)
	if err != nil {
		return reviewSubmission{}, err
	}
	for _, sub := range subs {
		if sub.versionID == v.ID {
			return sub, nil
		}
	}
	e := clierr.New("version %s has no review submission to cancel (state %s)", v.Version, v.State)
	e.Code = "STATE_ERROR"
	return reviewSubmission{}, e
}

// CancelVersion withdraws the review submission that holds v.
func (s *Service) CancelVersion(ctx context.Context, app App, v Version) (SubmissionResult, error) {
	sub, err := s.findCancelable(ctx, app, v)
	if err != nil {
		return SubmissionResult{}, err
	}
	state, err := s.updateSubmission(ctx, sub.id, asc.ReviewSubmissionUpdateRequestDataAttributes{Canceled: asc.Value(true)})
	if err != nil {
		return SubmissionResult{}, fmt.Errorf("cancel review submission %s: %w", sub.id, err)
	}
	return SubmissionResult{Version: v.Version, SubmissionID: sub.id, State: state, Items: sub.items}, nil
}

// PreviewCancelVersion finds the submission CancelVersion would withdraw,
// without changing anything.
func (s *Service) PreviewCancelVersion(ctx context.Context, app App, v Version) (SubmissionResult, error) {
	sub, err := s.findCancelable(ctx, app, v)
	if err != nil {
		return SubmissionResult{}, err
	}
	return SubmissionResult{Version: v.Version, SubmissionID: sub.id, State: stateDryRun, Items: sub.items}, nil
}

// updateSubmission patches a review submission and returns its new state.
func (s *Service) updateSubmission(ctx context.Context, id string, attrs asc.ReviewSubmissionUpdateRequestDataAttributes) (string, error) {
	resp, err := s.client.ReviewSubmissionsUpdateInstance(ctx, asc.ReviewSubmissionsUpdateInstanceParams{ID: id}, asc.ReviewSubmissionUpdateRequest{
		Data: asc.ReviewSubmissionUpdateRequestData{Type: asc.ResourceTypeReviewSubmissions, ID: id, Attributes: &attrs},
	})
	if err != nil {
		return "", err
	}
	if resp.Data.Attributes != nil {
		return string(deref(resp.Data.Attributes.State)), nil
	}
	return "", nil
}
