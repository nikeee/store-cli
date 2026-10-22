package cli_test

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/testutil/playfake"
)

const (
	bundleUploadPath = "/upload/androidpublisher/v3/applications/{packageName}/edits/edit-1/bundles"
	mappingPath      = "/upload/androidpublisher/v3/applications/{packageName}/edits/edit-1/apks/42/deobfuscationFiles/proguard"
	editTrackPath    = googleEditsPath + "/edit-1/tracks/{track}"
)

func bundleCLIServer(t *testing.T) *playfake.Server {
	srv := googleEditServer(t)
	srv.Handle("POST", bundleUploadPath, playfake.JSON(http.StatusOK, "bundle.json"))
	srv.Handle("GET", googleEditsPath+"/edit-1/listings", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"listings":[{"language":"en-US"}]}`)
	})
	srv.Handle("GET", editTrackPath, func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, `{"track":"`+r.PathValue("track")+`"}`)
	})
	srv.Handle("PUT", editTrackPath, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_, _ = w.Write(b)
	})
	return srv
}

func aabFile(t *testing.T) string {
	p := filepath.Join(t.TempDir(), "app.aab")
	if err := os.WriteFile(p, []byte("aab"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func bundleArgs(args ...string) []string {
	return append([]string{"google", "bundle", "upload", "--app", "com.example.app"}, args...)
}

func methods(srv *playfake.Server, method string) int {
	n := 0
	for _, r := range srv.Requests() {
		if r.Method == method {
			n++
		}
	}
	return n
}

func TestBundleUploadMultipleTracks(t *testing.T) {
	srv := bundleCLIServer(t)
	code, stdout, stderr := runEnv(t, googleEnv(srv, nil), false,
		bundleArgs(aabFile(t), "--track", "internal", "--track", "alpha", "--track", "internal", "-o", "json")...)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	var res struct {
		VersionCode int64
		Tracks      []struct{ Track string }
	}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil || res.VersionCode != 42 || len(res.Tracks) != 2 || res.Tracks[1].Track != "alpha" {
		t.Fatalf("stdout=%q err=%v", stdout, err)
	}
	if methods(srv, "PUT") != 2 || methods(srv, "POST") != 3 { // insert, upload, commit
		t.Errorf("requests = %d PUT, %d POST", methods(srv, "PUT"), methods(srv, "POST"))
	}
}

func TestBundleUploadDraftAppHint(t *testing.T) {
	srv := bundleCLIServer(t)
	srv.Handle("PUT", editTrackPath, playfake.JSON(http.StatusBadRequest, "error_draft_app.json"))
	code, _, stderr := runEnv(t, googleEnv(srv, nil), false, bundleArgs(aabFile(t), "--track", "internal", "-o", "json")...)
	if code != 1 || !strings.Contains(stderr, "use --status draft until the app is published") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if methods(srv, "DELETE") != 1 {
		t.Error("edit not deleted")
	}
}

func TestBundleUploadRolloutValidation(t *testing.T) {
	aab := aabFile(t)
	for _, args := range [][]string{
		{aab},                                   // no track
		{aab, "--track", "x", "--rollout", "0"}, // not in (0,1)
		{aab, "--track", "x", "--rollout", "1"},
		{aab, "--track", "x", "--rollout", "0.5", "--status", "draft"},
		{aab, "--track", "x", "--status", "done"},
	} {
		srv := bundleCLIServer(t)
		code, _, stderr := runEnv(t, googleEnv(srv, nil), false, bundleArgs(args...)...)
		if code != 2 || len(srv.Requests()) != 0 {
			t.Errorf("%v: code=%d stderr=%q requests=%d", args, code, stderr, len(srv.Requests()))
		}
	}
	srv := bundleCLIServer(t)
	code, _, stderr := runEnv(t, googleEnv(srv, nil), false, bundleArgs(aab, "--track", "production", "--rollout", "0.2", "-o", "json")...)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	var put map[string]any
	for _, r := range srv.Requests() {
		if r.Method == "PUT" {
			_ = json.Unmarshal(r.Body, &put)
		}
	}
	rel := put["releases"].([]any)[0].(map[string]any)
	if rel["status"] != "inProgress" || rel["userFraction"] != 0.2 {
		t.Errorf("release = %v", rel)
	}
}

func TestBundleUploadLocalValidationSendsNothing(t *testing.T) {
	dir := t.TempDir()
	long := filepath.Join(dir, "en-US.txt")
	if err := os.WriteFile(long, []byte(strings.Repeat("x", 501)), 0o600); err != nil {
		t.Fatal(err)
	}
	aab := aabFile(t)
	for name, args := range map[string][]string{
		"missing aab":     {filepath.Join(dir, "none.aab"), "--track", "x"},
		"missing mapping": {aab, "--track", "x", "--mapping", filepath.Join(dir, "none.txt")},
		"notes dir":       {aab, "--track", "x", "--release-notes", filepath.Join(dir, "none")},
		"notes too long":  {aab, "--track", "x", "--release-notes", dir},
	} {
		srv := bundleCLIServer(t)
		code, _, stderr := runEnv(t, googleEnv(srv, nil), false, bundleArgs(args...)...)
		if code != 1 || len(srv.Requests()) != 0 {
			t.Errorf("%s: code=%d stderr=%q requests=%d", name, code, stderr, len(srv.Requests()))
		}
	}
}

func TestBundleUploadMapping(t *testing.T) {
	srv := bundleCLIServer(t)
	var got string
	srv.Handle("POST", mappingPath, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		got = string(b)
		_, _ = io.WriteString(w, `{}`)
	})
	mapping := filepath.Join(t.TempDir(), "mapping.txt")
	if err := os.WriteFile(mapping, []byte("a -> b"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, _, stderr := runEnv(t, googleEnv(srv, nil), false, bundleArgs(aabFile(t), "--track", "internal", "--mapping", mapping)...)
	if code != 0 || got != "a -> b" {
		t.Fatalf("code=%d stderr=%q mapping=%q", code, stderr, got)
	}
}

func TestBundleUploadDryRun(t *testing.T) {
	srv := bundleCLIServer(t)
	code, stdout, stderr := runEnv(t, googleEnv(srv, nil), false, bundleArgs(aabFile(t), "--track", "internal", "--dry-run", "-o", "json")...)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	for _, r := range srv.Requests() {
		if r.Method == "PUT" || strings.Contains(r.Path, "/upload/") || strings.HasSuffix(r.Path, ":commit") {
			t.Errorf("mutating request %s %s", r.Method, r.Path)
		}
	}
	if methods(srv, "DELETE") != 1 || !strings.Contains(stdout, `"versionCode":0`) && !strings.Contains(stdout, `"versionCode": 0`) {
		t.Errorf("stdout=%q deletes=%d", stdout, methods(srv, "DELETE"))
	}
}
