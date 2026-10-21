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
