package cli_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/testutil/ascfake"
	"github.com/nikeee/store-cli/internal/testutil/golden"
)

func notesCLIServer(t *testing.T) *ascfake.Server {
	t.Helper()
	srv := buildServer(t)
	srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, "builds_valid.json"))
	srv.Handle("GET", "/v1/builds/{id}/betaBuildLocalizations", ascfake.JSON(http.StatusOK, "beta_build_localizations_one.json"))
	srv.Handle("PATCH", "/v1/betaBuildLocalizations/{id}", ascfake.JSON(http.StatusOK, "beta_build_localization.json"))
	return srv
}

func TestAppleTestflightNotesSetGolden(t *testing.T) {
	for _, mode := range []string{"text", "json", "toon"} {
		t.Run(mode, func(t *testing.T) {
			srv := notesCLIServer(t)
			code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "testflight", "notes", "set", "45", "--text", "Fixed login", "-o", mode)
			if code != 0 || stderr != "" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			golden.Assert(t, "apple_testflight_notes_set."+mode, []byte(stdout))
		})
	}
}

func TestAppleTestflightNotesSetRequiresText(t *testing.T) {
	srv := notesCLIServer(t)
	code, _, stderr := runEnv(t, buildEnv(srv), false, "apple", "testflight", "notes", "set", "45")
	if code != 2 || !strings.Contains(stderr, "--text") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestAppleTestflightNotesSetTooLongSendsNothing(t *testing.T) {
	srv := notesCLIServer(t)
	code, _, stderr := runEnv(t, buildEnv(srv), false, "apple", "testflight", "notes", "set", "45", "--text", strings.Repeat("x", 4001))
	if code != 1 || !strings.Contains(stderr, "4000") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if n := len(srv.Requests()); n != 0 {
		t.Errorf("sent %d requests", n)
	}
}

func TestAppleTestflightNotesSetDryRun(t *testing.T) {
	srv := notesCLIServer(t)
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "testflight", "notes", "set", "45", "--text", "Fixed login", "--dry-run", "-o", "json")
	if code != 0 || !strings.Contains(stdout, `"text": "Fixed login"`) {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	var sawLocalizations bool
	for _, r := range srv.Requests() {
		if r.Method != http.MethodGet {
			t.Errorf("dry run sent %s %s", r.Method, r.Path)
		}
		sawLocalizations = sawLocalizations || strings.HasSuffix(r.Path, "/betaBuildLocalizations")
	}
	if !sawLocalizations {
		t.Error("dry run did not look up the existing localization")
	}
}

func distServer(t *testing.T) *ascfake.Server {
	t.Helper()
	srv := buildServer(t)
	srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, "builds_valid.json"))
	srv.Handle("GET", "/v1/apps/{id}/betaGroups", ascfake.JSON(http.StatusOK, "beta_groups.json"))
	srv.Handle("GET", "/v1/betaTesters", ascfake.JSON(http.StatusOK, "beta_testers.json"))
	srv.Handle("GET", "/v1/builds/{id}/betaAppReviewSubmission", ascfake.JSON(http.StatusOK, "beta_app_review_submission_none.json"))
	srv.Handle("POST", "/v1/betaGroups", ascfake.JSON(http.StatusCreated, "beta_group.json"))
	srv.Handle("POST", "/v1/betaGroups/{id}/relationships/builds", ascfake.JSON(http.StatusNoContent, "empty.json"))
	srv.Handle("POST", "/v1/betaGroups/{id}/relationships/betaTesters", ascfake.JSON(http.StatusNoContent, "empty.json"))
	srv.Handle("DELETE", "/v1/betaGroups/{id}/relationships/betaTesters", ascfake.JSON(http.StatusNoContent, "empty.json"))
	srv.Handle("POST", "/v1/betaTesters", ascfake.JSON(http.StatusCreated, "beta_tester.json"))
	srv.Handle("POST", "/v1/betaAppReviewSubmissions", ascfake.JSON(http.StatusCreated, "beta_app_review_submission.json"))
	return srv
}

func TestAppleTestflightListGoldens(t *testing.T) {
	for _, c := range []struct {
		name, golden string
		args         []string
	}{
		{"group", "apple_testflight_group_list", []string{"group", "list"}},
		{"tester", "apple_testflight_tester_list", []string{"tester", "list"}},
	} {
		for _, mode := range []string{"text", "json", "toon"} {
			t.Run(c.name+"/"+mode, func(t *testing.T) {
				args := append([]string{"apple", "testflight"}, c.args...)
				code, stdout, stderr := runEnv(t, buildEnv(distServer(t)), false, append(args, "-o", mode)...)
				if code != 0 || stderr != "" {
					t.Fatalf("code=%d stderr=%q", code, stderr)
				}
				golden.Assert(t, c.golden+"."+mode, []byte(stdout))
			})
		}
	}
}

func TestAppleTestflightDistributionDryRun(t *testing.T) {
	for _, args := range [][]string{
		{"group", "create", "Friends", "--public-link"},
		{"group", "add-build", "Beta", "45"},
		{"tester", "add", "new@example.com", "--group", "Beta"},
		{"tester", "remove", "ada@example.com", "--group", "Beta"},
		{"submit", "45"},
	} {
		t.Run(args[0]+" "+args[1], func(t *testing.T) {
			srv := distServer(t)
			full := append([]string{"apple", "testflight"}, args...)
			code, _, stderr := runEnv(t, buildEnv(srv), false, append(full, "--dry-run")...)
			if code != 0 {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			for _, r := range srv.Requests() {
				if r.Method != http.MethodGet {
					t.Errorf("dry run sent %s %s", r.Method, r.Path)
				}
			}
		})
	}
}

func TestAppleTestflightTesterAddRequiresGroup(t *testing.T) {
	code, _, stderr := runEnv(t, buildEnv(distServer(t)), false, "apple", "testflight", "tester", "add", "a@example.com")
	if code != 2 || !strings.Contains(stderr, "--group") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestAppleTestflightSubmitAlreadySubmitted(t *testing.T) {
	srv := distServer(t)
	srv.Handle("POST", "/v1/betaAppReviewSubmissions", ascfake.JSON(http.StatusConflict, "error_409_beta_review.json"))
	srv.Handle("GET", "/v1/builds/{id}/betaAppReviewSubmission", ascfake.JSON(http.StatusOK, "beta_app_review_submission_in_review.json"))
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "testflight", "submit", "45", "-o", "json")
	if code != 0 || !strings.Contains(stdout, `"state": "in_review"`) {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestAppleTestflightSubmit409WithoutSubmissionFails(t *testing.T) {
	srv := distServer(t)
	srv.Handle("POST", "/v1/betaAppReviewSubmissions", ascfake.JSON(http.StatusConflict, "error_409_beta_review.json"))
	srv.Handle("GET", "/v1/builds/{id}/betaAppReviewSubmission", ascfake.JSON(http.StatusOK, "beta_app_review_submission_none.json"))
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "testflight", "submit", "45")
	if code != 1 || stdout != "" || !strings.Contains(stderr, "submit build 45 to Beta App Review") ||
		!strings.Contains(stderr, "The build has already been submitted for Beta App Review.") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}
