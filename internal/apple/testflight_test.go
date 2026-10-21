package apple

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
)

var testGroup = BetaGroup{ID: "grp-beta", Name: "Beta"}

// bodyOf decodes a recorded request body.
func bodyOf(t *testing.T, r ascfake.Request) map[string]any {
	t.Helper()
	var m map[string]any
	if err := json.Unmarshal(r.Body, &m); err != nil {
		t.Fatalf("body %q: %v", r.Body, err)
	}
	return m
}

// wantBody compares a recorded body with the JSON text want.
func wantBody(t *testing.T, r ascfake.Request, want string) {
	t.Helper()
	var w map[string]any
	if err := json.Unmarshal([]byte(want), &w); err != nil {
		t.Fatal(err)
	}
	if a, b := mustJSON(bodyOf(t, r)), mustJSON(w); a != b {
		t.Errorf("body = %s, want %s", a, b)
	}
}

func TestListGroups(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/apps/{id}/betaGroups", ascfake.JSON(http.StatusOK, "beta_groups.json"))
	got, err := newService(t, srv).ListGroups(t.Context(), testApp)
	if err != nil {
		t.Fatal(err)
	}
	want := []BetaGroup{
		{ID: "grp-internal", Name: "Team", Internal: true},
		{ID: "grp-beta", Name: "Beta", PublicLinkEnabled: true, PublicLink: "https://testflight.apple.com/join/AbCdEf"},
	}
	if mustJSON(got) != mustJSON(want) {
		t.Errorf("groups = %s, want %s", mustJSON(got), mustJSON(want))
	}
	if r := srv.Requests()[0]; r.Path != "/v1/apps/1234567890/betaGroups" {
		t.Errorf("path = %s", r.Path)
	}
}

func TestCreateGroup(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("POST", "/v1/betaGroups", ascfake.JSON(http.StatusCreated, "beta_group.json"))
	got, err := newService(t, srv).CreateGroup(t.Context(), testApp, GroupSpec{Name: "Friends", PublicLink: true, PublicLinkLimit: 50}, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "grp-new" || got.Name != "Friends" || got.PublicLink == "" {
		t.Errorf("group = %+v", got)
	}
	reqs := srv.Requests()
	if len(reqs) != 1 || reqs[0].Method != "POST" || reqs[0].Path != "/v1/betaGroups" {
		t.Fatalf("requests = %+v", reqs)
	}
	wantBody(t, reqs[0], `{"data":{"type":"betaGroups","attributes":{"name":"Friends","isInternalGroup":false,"publicLinkEnabled":true,"publicLinkLimitEnabled":true,"publicLinkLimit":50},"relationships":{"app":{"data":{"type":"apps","id":"1234567890"}}}}}`)
}

func TestCreateGroupInternal(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("POST", "/v1/betaGroups", ascfake.JSON(http.StatusCreated, "beta_group.json"))
	if _, err := newService(t, srv).CreateGroup(t.Context(), testApp, GroupSpec{Name: "Team", Internal: true}, false); err != nil {
		t.Fatal(err)
	}
	wantBody(t, srv.Requests()[0], `{"data":{"type":"betaGroups","attributes":{"name":"Team","isInternalGroup":true,"publicLinkEnabled":false},"relationships":{"app":{"data":{"type":"apps","id":"1234567890"}}}}}`)
}

func TestAddBuildToGroup(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("POST", "/v1/betaGroups/{id}/relationships/builds", ascfake.JSON(http.StatusNoContent, "empty.json"))
	got, err := newService(t, srv).AddBuildToGroup(t.Context(), testGroup, testBuild, false)
	if err != nil {
		t.Fatal(err)
	}
	if got != (GroupBuild{Group: "Beta", Build: "44"}) {
		t.Errorf("result = %+v", got)
	}
	reqs := srv.Requests()
	if len(reqs) != 1 || reqs[0].Path != "/v1/betaGroups/grp-beta/relationships/builds" {
		t.Fatalf("requests = %+v", reqs)
	}
	wantBody(t, reqs[0], `{"data":[{"type":"builds","id":"`+testBuild.ID+`"}]}`)
}

func TestResolveGroup(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/apps/{id}/betaGroups", ascfake.JSON(http.StatusOK, "beta_groups.json"))
	svc := newService(t, srv)
	for _, ref := range []string{"Beta", "grp-beta"} {
		g, err := svc.ResolveGroup(t.Context(), testApp, ref)
		if err != nil || g.ID != "grp-beta" {
			t.Errorf("ResolveGroup(%q) = %+v, %v", ref, g, err)
		}
	}
}

func TestResolveGroupNotFoundListsNames(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/apps/{id}/betaGroups", ascfake.JSON(http.StatusOK, "beta_groups.json"))
	_, err := newService(t, srv).ResolveGroup(t.Context(), testApp, "beta")
	if err == nil || clierr.ExitCode(err) != 1 || !strings.Contains(err.Error(), `no beta group "beta"; existing groups: Team, Beta`) {
		t.Fatalf("err = %v", err)
	}
}

func TestListTesters(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/betaTesters", ascfake.JSON(http.StatusOK, "beta_testers.json"))
	got, err := newService(t, srv).ListTesters(t.Context(), testApp, "grp-beta")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != (Tester{ID: "tester-1", Email: "ada@example.com", FirstName: "Ada", LastName: "Lovelace", InviteType: "email", State: "accepted"}) {
		t.Errorf("testers = %+v", got)
	}
	q := srv.Requests()[0].Query
	if q.Get("filter[apps]") != "1234567890" || q.Get("filter[betaGroups]") != "grp-beta" {
		t.Errorf("query = %s", q.Encode())
	}
}

func TestListTestersWithoutGroup(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/betaTesters", ascfake.JSON(http.StatusOK, "beta_testers.json"))
	if _, err := newService(t, srv).ListTesters(t.Context(), testApp, ""); err != nil {
		t.Fatal(err)
	}
	if q := srv.Requests()[0].Query; q.Has("filter[betaGroups]") {
		t.Errorf("query = %s", q.Encode())
	}
}

// testerServer answers the lookups of tester add: by e-mail within the group
// (inGroup) and by e-mail alone (exists).
func testerServer(t *testing.T, inGroup, exists string) *ascfake.Server {
	t.Helper()
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/betaTesters", func(w http.ResponseWriter, r *http.Request) {
		fixture := exists
		if r.URL.Query().Has("filter[betaGroups]") {
			fixture = inGroup
		}
		ascfake.JSON(http.StatusOK, fixture)(w, r)
	})
	srv.Handle("POST", "/v1/betaTesters", ascfake.JSON(http.StatusCreated, "beta_tester.json"))
	srv.Handle("POST", "/v1/betaGroups/{id}/relationships/betaTesters", ascfake.JSON(http.StatusNoContent, "empty.json"))
	srv.Handle("DELETE", "/v1/betaGroups/{id}/relationships/betaTesters", ascfake.JSON(http.StatusNoContent, "empty.json"))
	return srv
}

func TestAddTesterCreates(t *testing.T) {
	srv := testerServer(t, "beta_testers_empty.json", "beta_testers_empty.json")
	got, err := newService(t, srv).AddTester(t.Context(), testGroup, "grace@example.com", "Grace", "Hopper", false)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "tester-3" || got.Email != "grace@example.com" {
		t.Errorf("tester = %+v", got)
	}
	reqs := srv.Requests()
	last := reqs[len(reqs)-1]
	if last.Method != "POST" || last.Path != "/v1/betaTesters" {
		t.Fatalf("requests = %+v", reqs)
	}
	wantBody(t, last, `{"data":{"type":"betaTesters","attributes":{"email":"grace@example.com","firstName":"Grace","lastName":"Hopper"},"relationships":{"betaGroups":{"data":[{"type":"betaGroups","id":"grp-beta"}]}}}}`)
}

func TestAddTesterExistingJoinsGroup(t *testing.T) {
	srv := testerServer(t, "beta_testers_empty.json", "beta_testers_one.json")
	got, err := newService(t, srv).AddTester(t.Context(), testGroup, "ada@example.com", "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "tester-1" {
		t.Errorf("tester = %+v", got)
	}
	reqs := srv.Requests()
	last := reqs[len(reqs)-1]
	if last.Method != "POST" || last.Path != "/v1/betaGroups/grp-beta/relationships/betaTesters" {
		t.Fatalf("requests = %+v", reqs)
	}
	wantBody(t, last, `{"data":[{"type":"betaTesters","id":"tester-1"}]}`)
}

func TestTesterAddIdempotent(t *testing.T) {
	srv := testerServer(t, "beta_testers_one.json", "beta_testers_one.json")
	got, err := newService(t, srv).AddTester(t.Context(), testGroup, "ada@example.com", "", "", false)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "tester-1" {
		t.Errorf("tester = %+v", got)
	}
	for _, r := range srv.Requests() {
		if r.Method != "GET" {
			t.Errorf("already a member, but sent %s %s", r.Method, r.Path)
		}
	}
}

func TestRemoveTester(t *testing.T) {
	srv := testerServer(t, "beta_testers_one.json", "beta_testers_one.json")
	got, err := newService(t, srv).RemoveTester(t.Context(), testGroup, "ada@example.com", false)
	if err != nil {
		t.Fatal(err)
	}
	if got.ID != "tester-1" {
		t.Errorf("tester = %+v", got)
	}
	reqs := srv.Requests()
	last := reqs[len(reqs)-1]
	if last.Method != "DELETE" || last.Path != "/v1/betaGroups/grp-beta/relationships/betaTesters" {
		t.Fatalf("requests = %+v", reqs)
	}
	wantBody(t, last, `{"data":[{"type":"betaTesters","id":"tester-1"}]}`)
}

func TestRemoveTesterNotInGroup(t *testing.T) {
	srv := testerServer(t, "beta_testers_empty.json", "beta_testers_one.json")
	_, err := newService(t, srv).RemoveTester(t.Context(), testGroup, "ada@example.com", false)
	if err == nil || clierr.ExitCode(err) != 1 || !strings.Contains(err.Error(), `ada@example.com is not in group "Beta"`) {
		t.Fatalf("err = %v", err)
	}
	for _, r := range srv.Requests() {
		if r.Method != "GET" {
			t.Errorf("sent %s %s", r.Method, r.Path)
		}
	}
}

func TestSubmitBetaReview(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("POST", "/v1/betaAppReviewSubmissions", ascfake.JSON(http.StatusCreated, "beta_app_review_submission.json"))
	got, err := newService(t, srv).SubmitBetaReview(t.Context(), testBuild, false)
	if err != nil {
		t.Fatal(err)
	}
	if got != (BetaReview{Build: "44", State: "waiting_for_review"}) {
		t.Errorf("result = %+v", got)
	}
	reqs := srv.Requests()
	if len(reqs) != 1 || reqs[0].Path != "/v1/betaAppReviewSubmissions" {
		t.Fatalf("requests = %+v", reqs)
	}
	wantBody(t, reqs[0], `{"data":{"type":"betaAppReviewSubmissions","relationships":{"build":{"data":{"type":"builds","id":"`+testBuild.ID+`"}}}}}`)
}

func TestBetaReviewAlreadySubmitted(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("POST", "/v1/betaAppReviewSubmissions", ascfake.JSON(http.StatusConflict, "error_409_beta_review.json"))
	srv.Handle("GET", "/v1/builds/{id}/betaAppReviewSubmission", ascfake.JSON(http.StatusOK, "beta_app_review_submission_in_review.json"))
	got, err := newService(t, srv).SubmitBetaReview(t.Context(), testBuild, false)
	if err != nil {
		t.Fatal(err)
	}
	if got != (BetaReview{Build: "44", State: "in_review"}) {
		t.Errorf("result = %+v", got)
	}
	if r := srv.Requests()[1]; r.Path != "/v1/builds/"+testBuild.ID+"/betaAppReviewSubmission" {
		t.Errorf("lookup path = %s", r.Path)
	}
}

func TestBetaReviewOther409IsStillAlreadySubmittedWithoutState(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("POST", "/v1/betaAppReviewSubmissions", ascfake.JSON(http.StatusConflict, "error_409_beta_review.json"))
	srv.Handle("GET", "/v1/builds/{id}/betaAppReviewSubmission", ascfake.JSON(http.StatusOK, "beta_app_review_submission_none.json"))
	got, err := newService(t, srv).SubmitBetaReview(t.Context(), testBuild, false)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "submitted" {
		t.Errorf("result = %+v", got)
	}
}

func TestBetaReviewOtherErrorFails(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("POST", "/v1/betaAppReviewSubmissions", ascfake.JSON(http.StatusUnauthorized, "error_401.json"))
	if _, err := newService(t, srv).SubmitBetaReview(t.Context(), testBuild, false); err == nil {
		t.Fatal("expected an error")
	}
}
