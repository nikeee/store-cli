package cli_test

import (
	"net/http"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/testutil/ascfake"
	"github.com/nikeee/store-cli/internal/testutil/golden"
)

func versionCLIServer(t *testing.T, fixture string) *ascfake.Server {
	t.Helper()
	srv := buildServer(t)
	srv.Handle("GET", "/v1/apps/{id}/appStoreVersions", ascfake.JSON(http.StatusOK, fixture))
	return srv
}

func TestAppleVersionListGolden(t *testing.T) {
	for _, mode := range []string{"text", "json", "toon"} {
		t.Run(mode, func(t *testing.T) {
			srv := versionCLIServer(t, "app_appStoreVersions.json")
			code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "version", "list", "-o", mode)
			if code != 0 || stderr != "" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			golden.Assert(t, "apple_version_list."+mode, []byte(stdout))
			q := srv.Requests()[1].Query
			if q.Get("limit") != "10" || q.Get("filter[platform]") != "IOS" {
				t.Errorf("query = %s", q.Encode())
			}
		})
	}
}

func TestAppleVersionShowGolden(t *testing.T) {
	for _, mode := range []string{"text", "json", "toon"} {
		t.Run(mode, func(t *testing.T) {
			srv := versionCLIServer(t, "app_appStoreVersions.json")
			code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "version", "show", "1.3.0", "-o", mode)
			if code != 0 || stderr != "" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			golden.Assert(t, "apple_version_show."+mode, []byte(stdout))
		})
	}
}

func TestAppleVersionShowNotFound(t *testing.T) {
	srv := versionCLIServer(t, "app_appStoreVersions.json")
	code, _, stderr := runEnv(t, buildEnv(srv), false, "apple", "version", "show", "9.9")
	if code != 1 || !strings.Contains(stderr, "version 9.9 not found") || !strings.Contains(stderr, "store apple version list") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestAppleVersionCreate(t *testing.T) {
	srv := versionCLIServer(t, "versions_live_only.json")
	srv.Handle("POST", "/v1/appStoreVersions", ascfake.JSON(http.StatusCreated, "version_created.json"))
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "version", "create", "1.4.0",
		"--release", "scheduled", "--release-date", "2026-12-01T09:00:00Z", "--copyright", "2026 Example GmbH", "-o", "json")
	if code != 0 || !strings.Contains(stdout, `"version": "1.4.0"`) {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	reqs := srv.Requests()
	body := string(reqs[len(reqs)-1].Body)
	if !strings.Contains(body, `"releaseType":"SCHEDULED"`) || !strings.Contains(body, `"copyright":"2026 Example GmbH"`) {
		t.Errorf("body = %s", body)
	}
}

func TestAppleVersionCreateUsageErrors(t *testing.T) {
	for name, args := range map[string][]string{
		"scheduled without date": {"--release", "scheduled"},
		"bad date":               {"--release", "scheduled", "--release-date", "tomorrow"},
		"bad release":            {"--release", "now"},
		"date without release":   {"--release-date", "2026-12-01T09:00:00Z"},
		"date with manual":       {"--release", "manual", "--release-date", "2026-12-01T09:00:00Z"},
	} {
		t.Run(name, func(t *testing.T) {
			srv := versionCLIServer(t, "versions_live_only.json")
			code, _, stderr := runEnv(t, buildEnv(srv), false, append([]string{"apple", "version", "create", "1.4.0"}, args...)...)
			if code != 2 || !strings.Contains(stderr, "--release") {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			if n := len(srv.Requests()); n != 0 {
				t.Errorf("requests = %d", n)
			}
		})
	}
}

func TestAppleVersionSetBuild(t *testing.T) {
	srv := versionCLIServer(t, "app_appStoreVersions.json")
	srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, "builds_valid.json"))
	srv.Handle("PATCH", "/v1/appStoreVersions/{id}/relationships/build", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "version", "set-build", "1.3.0", "45", "-o", "json")
	if code != 0 || !strings.Contains(stdout, `"build": "45"`) {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	var sawBuildQuery bool
	for _, r := range srv.Requests() {
		if r.Path == "/v1/builds" {
			sawBuildQuery = r.Query.Get("filter[preReleaseVersion.version]") == "1.3.0"
		}
	}
	if !sawBuildQuery {
		t.Error("build not looked up under the version being edited")
	}
}

func TestAppleVersionMutationsDryRun(t *testing.T) {
	for _, args := range [][]string{
		{"apple", "version", "create", "1.4.0", "--dry-run", "-o", "json"},
		{"apple", "version", "set-build", "1.3.0", "45", "--dry-run", "-o", "json"},
	} {
		srv := versionCLIServer(t, "app_appStoreVersions.json")
		if args[2] == "create" {
			srv = versionCLIServer(t, "versions_live_only.json")
		}
		srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, "builds_valid.json"))
		code, stdout, stderr := runEnv(t, buildEnv(srv), false, args...)
		if code != 0 {
			t.Fatalf("%v: code=%d stderr=%q", args, code, stderr)
		}
		want := `"version": "1.4.0"`
		if args[2] == "set-build" {
			want = `"build": "45"`
		}
		if !strings.Contains(stdout, want) {
			t.Errorf("%v: stdout = %s", args, stdout)
		}
		for _, r := range srv.Requests() {
			if r.Method != http.MethodGet {
				t.Errorf("%v: dry run sent %s %s", args, r.Method, r.Path)
			}
		}
	}
}

func submitCLIServer(t *testing.T, list string) *ascfake.Server {
	t.Helper()
	srv := versionCLIServer(t, "app_appStoreVersions.json")
	srv.Handle("GET", "/v1/apps/{id}/reviewSubmissions", ascfake.JSON(http.StatusOK, list))
	srv.Handle("POST", "/v1/reviewSubmissions", ascfake.JSON(http.StatusCreated, "review_submission_created.json"))
	srv.Handle("POST", "/v1/reviewSubmissionItems", ascfake.JSON(http.StatusCreated, "review_submission_item_created.json"))
	srv.Handle("PATCH", "/v1/reviewSubmissions/{id}", ascfake.JSON(http.StatusOK, "review_submission_submitted.json"))
	return srv
}

func TestAppleVersionSubmit(t *testing.T) {
	srv := submitCLIServer(t, "review_submissions_none.json")
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "version", "submit", "1.3.0", "-o", "json")
	if code != 0 || !strings.Contains(stdout, `"submissionId": "5c1b7e30-0a42-4d9f-8e11-3b6f2a9d7c01"`) || !strings.Contains(stdout, `"state": "WAITING_FOR_REVIEW"`) || !strings.Contains(stdout, `"items": 1`) {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestAppleVersionSubmitNotFound(t *testing.T) {
	srv := submitCLIServer(t, "review_submissions_none.json")
	code, _, stderr := runEnv(t, buildEnv(srv), false, "apple", "version", "submit", "9.9")
	if code != 1 || !strings.Contains(stderr, "version 9.9 not found") || !strings.Contains(stderr, "store apple version create 9.9") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestAppleVersionCancel(t *testing.T) {
	srv := submitCLIServer(t, "review_submissions_waiting.json")
	srv.Handle("PATCH", "/v1/reviewSubmissions/{id}", ascfake.JSON(http.StatusOK, "review_submission_canceled.json"))
	srv.Handle("GET", "/v1/apps/{id}/appStoreVersions", ascfake.JSON(http.StatusOK, "app_appStoreVersions.json"))
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "version", "cancel", "1.3.0", "-o", "json")
	if code != 0 || !strings.Contains(stdout, `"state": "CANCELING"`) {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestAppleVersionSubmitCancelDryRun(t *testing.T) {
	for _, cmd := range []string{"submit", "cancel"} {
		list := "review_submissions_draft.json"
		if cmd == "cancel" {
			list = "review_submissions_waiting.json"
		}
		srv := submitCLIServer(t, list)
		code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "version", cmd, "1.3.0", "--dry-run", "-o", "json")
		if code != 0 || !strings.Contains(stdout, `"state": "dry-run"`) {
			t.Fatalf("%s: code=%d stdout=%q stderr=%q", cmd, code, stdout, stderr)
		}
		for _, r := range srv.Requests() {
			if r.Method != http.MethodGet {
				t.Errorf("%s: dry run sent %s %s", cmd, r.Method, r.Path)
			}
		}
	}
}

func releaseCLIServer(t *testing.T) *ascfake.Server {
	t.Helper()
	srv := versionCLIServer(t, "versions_pending_release.json")
	srv.Handle("GET", "/v1/appStoreVersions/{id}/appStoreVersionPhasedRelease", ascfake.JSON(http.StatusOK, "phased_release_active.json"))
	srv.Handle("POST", "/v1/appStoreVersionReleaseRequests", ascfake.JSON(http.StatusCreated, "release_request_created.json"))
	srv.Handle("POST", "/v1/appStoreVersionPhasedReleases", ascfake.JSON(http.StatusCreated, "phased_release_created_inactive.json"))
	srv.Handle("PATCH", "/v1/appStoreVersionPhasedReleases/{id}", ascfake.JSON(http.StatusOK, "phased_release_paused.json"))
	return srv
}

func TestAppleVersionRelease(t *testing.T) {
	srv := releaseCLIServer(t)
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "version", "release", "1.3.0", "-o", "json")
	if code != 0 || !strings.Contains(stdout, `"version": "1.3.0"`) {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if reqs := srv.Requests(); reqs[len(reqs)-1].Path != "/v1/appStoreVersionReleaseRequests" {
		t.Errorf("last request = %s", reqs[len(reqs)-1].Path)
	}
}

func TestAppleVersionReleaseWrongState(t *testing.T) {
	srv := versionCLIServer(t, "app_appStoreVersions.json")
	code, _, stderr := runEnv(t, buildEnv(srv), false, "apple", "version", "release", "1.3.0")
	if code != 1 || !strings.Contains(stderr, "PREPARE_FOR_SUBMISSION") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestAppleVersionPhased(t *testing.T) {
	srv := releaseCLIServer(t)
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "version", "phased", "pause", "1.3.0", "-o", "json")
	if code != 0 || !strings.Contains(stdout, `"state": "PAUSED"`) || !strings.Contains(stdout, `"currentDayNumber": 3`) {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestAppleVersionPhasedWithoutPhasedRelease(t *testing.T) {
	srv := releaseCLIServer(t)
	srv.Handle("GET", "/v1/appStoreVersions/{id}/appStoreVersionPhasedRelease", ascfake.JSON(http.StatusOK, "phased_release_none.json"))
	code, _, stderr := runEnv(t, buildEnv(srv), false, "apple", "version", "phased", "resume", "1.3.0")
	if code != 1 || !strings.Contains(stderr, "store apple version phased start 1.3.0") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestAppleVersionReleasePhasedDryRun(t *testing.T) {
	for _, args := range [][]string{
		{"apple", "version", "release", "1.3.0", "--dry-run", "-o", "json"},
		{"apple", "version", "phased", "pause", "1.3.0", "--dry-run", "-o", "json"},
		{"apple", "version", "phased", "resume", "1.3.0", "--dry-run", "-o", "json"},
		{"apple", "version", "phased", "complete", "1.3.0", "--dry-run", "-o", "json"},
		{"apple", "version", "phased", "start", "1.3.0", "--dry-run", "-o", "json"},
	} {
		srv := releaseCLIServer(t)
		if args[3] == "start" {
			srv.Handle("GET", "/v1/appStoreVersions/{id}/appStoreVersionPhasedRelease", ascfake.JSON(http.StatusOK, "phased_release_none.json"))
		}
		code, stdout, stderr := runEnv(t, buildEnv(srv), false, args...)
		if code != 0 || !strings.Contains(stdout, `"version": "1.3.0"`) {
			t.Fatalf("%v: code=%d stdout=%q stderr=%q", args, code, stdout, stderr)
		}
		for _, r := range srv.Requests() {
			if r.Method != http.MethodGet {
				t.Errorf("%v: dry run sent %s %s", args, r.Method, r.Path)
			}
		}
	}
}
