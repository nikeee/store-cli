package cli_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/testutil/ascfake"
)

func submitAllServer(t *testing.T, versions string) *ascfake.Server {
	t.Helper()
	srv := submitCLIServer(t, "review_submissions_none.json")
	srv.Handle("GET", "/v1/apps/{id}/appStoreVersions", ascfake.JSON(http.StatusOK, versions))
	srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, "builds_valid.json"))
	srv.Handle("GET", "/v1/apps/{id}/appInfos", ascfake.JSON(http.StatusOK, "app_appInfos.json"))
	srv.Handle("GET", "/v1/appInfos/{id}/appInfoLocalizations", ascfake.JSON(http.StatusOK, "appInfo_localizations_full.json"))
	srv.Handle("GET", "/v1/appStoreVersions/{id}/appStoreVersionLocalizations", ascfake.JSON(http.StatusOK, "version_localizations_en_only.json"))
	srv.Handle("PATCH", "/v1/appStoreVersionLocalizations/{id}", ascfake.JSON(http.StatusOK, "versionLocalization_created.json"))
	srv.Handle("PATCH", "/v1/appStoreVersions/{id}/relationships/build", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	return srv
}

func submitNotes(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "en-US.txt"), []byte("Fixes\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	return dir
}

func TestAppleSubmit(t *testing.T) {
	srv := submitAllServer(t, "app_appStoreVersions.json")
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "submit", "1.3.0", "--release-notes", submitNotes(t), "-o", "json")
	if code != 0 || !strings.Contains(stdout, `"build": "45"`) || !strings.Contains(stdout, `"state": "WAITING_FOR_REVIEW"`) || !strings.Contains(stdout, `"set release notes"`) {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestAppleSubmitDryRun(t *testing.T) {
	srv := submitAllServer(t, "app_appStoreVersions.json")
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "submit", "1.3.0", "--release-notes", submitNotes(t), "--dry-run", "-o", "json")
	if code != 0 || !strings.Contains(stdout, `"state": "dry-run"`) {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	for _, r := range srv.Requests() {
		if r.Method != http.MethodGet {
			t.Errorf("dry run sent %s %s", r.Method, r.Path)
		}
	}
}

func TestAppleSubmitInvalidInputSendsNothing(t *testing.T) {
	srv := submitAllServer(t, "app_appStoreVersions.json")
	for _, c := range []struct {
		args []string
		code int
	}{
		{[]string{"--release-notes", filepath.Join(t.TempDir(), "missing")}, 1},
		{[]string{"--metadata", filepath.Join(t.TempDir(), "missing")}, 1},
		{[]string{"--release", "scheduled"}, 2},
		{[]string{"--release", "later"}, 2},
		{[]string{"--release", "scheduled", "--release-date", "tomorrow"}, 2},
	} {
		args := append([]string{"apple", "submit", "1.3.0"}, c.args...)
		if code, _, stderr := runEnv(t, buildEnv(srv), false, args...); code != c.code {
			t.Errorf("%v: code=%d stderr=%q", c.args, code, stderr)
		}
	}
	if n := len(srv.Requests()); n != 0 {
		t.Errorf("requests = %d", n)
	}
}
