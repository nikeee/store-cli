package cli_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/config"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
	"github.com/nikeee/store-cli/internal/testutil/golden"
)

func buildEnv(srv *ascfake.Server) map[string]string {
	return appleEnv(srv, map[string]string{config.EnvAppleBundleID: "com.example.app"})
}

func buildServer(t *testing.T) *ascfake.Server {
	t.Helper()
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/apps", ascfake.JSON(http.StatusOK, "apps_one.json"))
	return srv
}

func TestAppleBuildListGolden(t *testing.T) {
	for _, mode := range []string{"text", "json", "toon"} {
		t.Run(mode, func(t *testing.T) {
			srv := buildServer(t)
			srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, "builds_list.json"))
			code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "build", "list", "-o", mode)
			if code != 0 || stderr != "" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			golden.Assert(t, "apple_build_list."+mode, []byte(stdout))
			q := srv.Requests()[1].Query
			if q.Get("limit") != "20" || q.Get("filter[preReleaseVersion.platform]") != "IOS" || q.Get("filter[app]") != "1234567890" {
				t.Errorf("query = %s", q.Encode())
			}
		})
	}
}

func TestAppleBuildListInvalidState(t *testing.T) {
	srv := buildServer(t)
	code, _, stderr := runEnv(t, buildEnv(srv), false, "apple", "build", "list", "--state", "done")
	if code != 2 || !strings.Contains(stderr, "--state") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestAppleBuildShowAmbiguousExits2(t *testing.T) {
	srv := buildServer(t)
	srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, "builds_ambiguous.json"))
	code, _, stderr := runEnv(t, buildEnv(srv), false, "apple", "build", "show", "42")
	if code != 2 || !strings.Contains(stderr, "build 42 exists for versions 1.0, 1.1; pass --version") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestAppleBuildWait(t *testing.T) {
	srv := buildServer(t)
	srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, "builds_valid.json"))
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "build", "wait", "45", "--timeout", "5m", "-o", "json")
	var b struct{ Build, State string }
	if err := json.Unmarshal([]byte(stdout), &b); err != nil || code != 0 || b.State != "valid" || b.Build != "45" {
		t.Fatalf("code=%d stdout=%q stderr=%q err=%v", code, stdout, stderr, err)
	}
}

func TestAppleBuildSetEncryption(t *testing.T) {
	srv := buildServer(t)
	srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, "builds_valid.json"))
	srv.Handle("PATCH", "/v1/builds/{id}", ascfake.JSON(http.StatusOK, "build_updated.json"))
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "build", "set-encryption", "45", "false", "-o", "json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, `"usesNonExemptEncryption": false`) || !strings.Contains(stdout, `"version": "1.3.0"`) {
		t.Errorf("stdout = %s", stdout)
	}
	code, _, stderr = runEnv(t, buildEnv(srv), false, "apple", "build", "set-encryption", "45", "maybe")
	if code != 2 {
		t.Errorf("code=%d stderr=%q", code, stderr)
	}
}

func TestAppleBuildMutationsDryRun(t *testing.T) {
	for _, args := range [][]string{
		{"apple", "build", "set-encryption", "45", "true", "--dry-run", "-o", "json"},
		{"apple", "build", "expire", "45", "--dry-run", "-o", "json"},
	} {
		srv := buildServer(t)
		srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, "builds_valid.json"))
		code, stdout, stderr := runEnv(t, buildEnv(srv), false, args...)
		if code != 0 {
			t.Fatalf("%v: code=%d stderr=%q", args, code, stderr)
		}
		want := `"usesNonExemptEncryption": true`
		if args[2] == "expire" {
			want = `"expired": true`
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
