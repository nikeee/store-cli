package cli_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/config"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
	"github.com/nikeee/store-cli/internal/testutil/golden"
	"github.com/nikeee/store-cli/internal/testutil/ipafake"
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

func uploadCLIServer(t *testing.T) *ascfake.Server {
	t.Helper()
	srv := buildServer(t)
	srv.Handle("POST", "/v1/buildUploads", ascfake.JSON(http.StatusCreated, "buildUploads_create.json"))
	srv.Handle("POST", "/v1/buildUploadFiles", ascfake.JSON(http.StatusCreated, "buildUploadFiles_create.json"))
	srv.Handle("PATCH", "/v1/buildUploadFiles/{id}", ascfake.JSON(http.StatusOK, "buildUploadFiles_update.json"))
	srv.Handle("GET", "/v1/buildUploads/{id}", ascfake.JSON(http.StatusOK, "buildUploads_get_complete.json"))
	return srv
}

func uploadIPA(t *testing.T) string {
	t.Helper()
	return ipafake.Write(t, ipafake.Spec{BundleID: "com.example.app", Version: "1.3.0", Build: "45", Size: 2000})
}

func TestAppleBuildUpload(t *testing.T) {
	srv := uploadCLIServer(t)
	srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, "builds_processing.json"))
	srv.Handle("GET", "/v1/builds/{id}/betaBuildLocalizations", ascfake.JSON(http.StatusOK, "beta_build_localizations_empty.json"))
	srv.Handle("POST", "/v1/betaBuildLocalizations", ascfake.JSON(http.StatusCreated, "beta_build_localization.json"))
	srv.Handle("PATCH", "/v1/builds/{id}", ascfake.JSON(http.StatusOK, "build_updated.json"))
	notes := filepath.Join(t.TempDir(), "notes.txt")
	if err := os.WriteFile(notes, []byte("Fixed login\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	// No --app: the app comes from the IPA.
	env := appleEnv(srv, nil)
	code, stdout, stderr := runEnv(t, env, false, "apple", "build", "upload", uploadIPA(t),
		"--notes", "@"+notes, "--uses-non-exempt-encryption", "false", "-o", "json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	var res map[string]string
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatal(err)
	}
	if res["app"] != "com.example.app" || res["buildUploadId"] != "bu-1" || res["buildId"] == "" || res["state"] != "complete" || res["build"] != "45" {
		t.Errorf("result = %v", res)
	}
	var sawNotes bool
	for _, r := range srv.Requests() {
		if r.Method == http.MethodPost && r.Path == "/v1/betaBuildLocalizations" {
			sawNotes = strings.Contains(string(r.Body), `"whatsNew":"Fixed login"`) && strings.Contains(string(r.Body), `"locale":"en-US"`)
		}
	}
	if !sawNotes {
		t.Error("notes not set from the file")
	}
}

// A step after the upload fails: exit 1, the error says the build was
// uploaded and names the next command; json mode also prints the result.
func TestAppleBuildUploadFailsAfterUpload(t *testing.T) {
	for _, mode := range []string{"text", "json"} {
		t.Run(mode, func(t *testing.T) {
			srv := uploadCLIServer(t)
			srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, "builds_processing.json"))
			srv.Handle("PATCH", "/v1/builds/{id}", ascfake.JSON(http.StatusConflict, "error_409.json"))
			code, stdout, stderr := runEnv(t, appleEnv(srv, nil), false, "apple", "build", "upload", uploadIPA(t),
				"--uses-non-exempt-encryption", "false", "-o", mode)
			if code != 1 || !strings.Contains(stderr, "build 45 (1.3.0) was uploaded (build upload bu-1); then: update build 45: An attribute value is invalid.") ||
				!strings.Contains(stderr, "store apple build set-encryption 45 false") {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			if mode == "text" {
				if stdout != "" {
					t.Errorf("stdout = %q", stdout)
				}
				return
			}
			var res map[string]string
			if err := json.Unmarshal([]byte(stdout), &res); err != nil {
				t.Fatalf("stdout = %q: %v", stdout, err)
			}
			if res["buildUploadId"] != "bu-1" || res["state"] != "complete" || res["buildId"] == "" {
				t.Errorf("result = %v", res)
			}
		})
	}
}

func TestAppleBuildUploadAppMismatchExits2(t *testing.T) {
	srv := uploadCLIServer(t)
	code, _, stderr := runEnv(t, appleEnv(srv, nil), false, "apple", "build", "upload", uploadIPA(t), "--app", "com.other.app")
	if code != 2 || !strings.Contains(stderr, "IPA has bundle ID com.example.app, --app is com.other.app") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if n := len(srv.Requests()); n != 0 {
		t.Errorf("%d requests sent", n)
	}
}

func TestAppleBuildUploadInvalidFlags(t *testing.T) {
	srv := uploadCLIServer(t)
	for _, args := range [][]string{
		{"--uses-non-exempt-encryption", "maybe"},
		{"--timeout", "0s"},
	} {
		code, _, stderr := runEnv(t, appleEnv(srv, nil), false, append([]string{"apple", "build", "upload", uploadIPA(t)}, args...)...)
		if code != 2 {
			t.Errorf("%v: code=%d stderr=%q", args, code, stderr)
		}
	}
	if n := len(srv.Requests()); n != 0 {
		t.Errorf("%d requests sent", n)
	}
}

func TestAppleBuildUploadDryRun(t *testing.T) {
	srv := uploadCLIServer(t)
	code, stdout, stderr := runEnv(t, appleEnv(srv, nil), false, "apple", "build", "upload", uploadIPA(t), "--dry-run", "--wait", "-o", "json")
	if code != 0 || !strings.Contains(stdout, `"state": "dry-run"`) || !strings.Contains(stdout, `"build": "45"`) {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	for _, r := range srv.Requests() {
		if r.Method != http.MethodGet {
			t.Errorf("dry run sent %s %s", r.Method, r.Path)
		}
	}
}
