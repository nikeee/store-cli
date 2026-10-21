package cli_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/metadata"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
	"github.com/nikeee/store-cli/internal/testutil/golden"
)

const reviewCLIDetail = `{"data":{"type":"appStoreReviewDetails","id":"rd-1","attributes":{"contactFirstName":"Ada","contactLastName":"Lovelace","contactPhone":"+1 555 0100","contactEmail":"ada@example.com","demoAccountRequired":true,"demoAccountName":"demo","demoAccountPassword":"s3cret","notes":"Use the demo."}},"links":{"self":"x"}}`

func reviewCLIServer(t *testing.T) *ascfake.Server {
	t.Helper()
	srv := versionCLIServer(t, "app_appStoreVersions.json")
	srv.Handle("GET", "/v1/appStoreVersions/{id}/appStoreReviewDetail", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(reviewCLIDetail))
	})
	srv.Handle("GET", "/v1/appStoreReviewDetails/{id}/appStoreReviewAttachments", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":[{"type":"appStoreReviewAttachments","id":"att-a","attributes":{"fileName":"login.pdf"}}],"links":{"self":"x"}}`))
	})
	srv.Handle("PATCH", "/v1/appStoreReviewDetails/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(reviewCLIDetail))
	})
	srv.Handle("DELETE", "/v1/appStoreReviewAttachments/{id}", ascfake.JSON(http.StatusNoContent, "empty.json"))
	srv.Handle("POST", "/v1/appStoreReviewAttachments", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_, _ = w.Write([]byte(`{"data":{"type":"appStoreReviewAttachments","id":"att-new","attributes":{"uploadOperations":[{"method":"PUT","url":"` + srv.UploadURL("att-new") + `","offset":0,"length":5}]}},"links":{"self":"x"}}`))
	})
	srv.Handle("PATCH", "/v1/appStoreReviewAttachments/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"type":"appStoreReviewAttachments","id":"att-new"},"links":{"self":"x"}}`))
	})
	srv.Handle("GET", "/v1/appStoreReviewAttachments/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"data":{"type":"appStoreReviewAttachments","id":"att-new","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}},"links":{"self":"x"}}`))
	})
	return srv
}

func TestAppleReviewInfoShowGolden(t *testing.T) {
	for _, mode := range []string{"text", "json", "toon"} {
		t.Run(mode, func(t *testing.T) {
			srv := reviewCLIServer(t)
			code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "review-info", "show", "-o", mode)
			if code != 0 || stderr != "" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			if strings.Contains(stdout, "s3cret") {
				t.Errorf("password shown: %s", stdout)
			}
			golden.Assert(t, "apple_review_info_show."+mode, []byte(stdout))
		})
	}
}

func TestAppleReviewInfoShowReveal(t *testing.T) {
	code, stdout, stderr := runEnv(t, buildEnv(reviewCLIServer(t)), false, "apple", "review-info", "show", "--reveal", "-o", "json")
	if code != 0 || !strings.Contains(stdout, `"demoPassword": "s3cret"`) {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestAppleReviewInfoSetFlagsFromTable(t *testing.T) {
	code, stdout, stderr := runEnv(t, buildEnv(reviewCLIServer(t)), false, "apple", "review-info", "set", "--help")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	for _, f := range metadata.AppleReviewFields {
		if !strings.Contains(stdout, "--"+f.Flag+" ") {
			t.Errorf("flag --%s missing", f.Flag)
		}
	}
	if !strings.Contains(stdout, "--attachment ") {
		t.Error("--attachment missing")
	}
}

func TestAppleReviewInfoSet(t *testing.T) {
	srv := reviewCLIServer(t)
	dir := t.TempDir()
	pw := filepath.Join(dir, "pw.txt")
	att := filepath.Join(dir, "flow.pdf")
	for p, c := range map[string]string{pw: "newpass", att: "bytes"} {
		if err := os.WriteFile(p, []byte(c), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "review-info", "set", "--demo-required", "false",
		"--demo-password", "@"+pw, "--notes", "", "--attachment", att, "-o", "json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if strings.Contains(stdout, "newpass") {
		t.Errorf("password in output: %s", stdout)
	}
	var body string
	var deleted bool
	for _, r := range srv.Requests() {
		switch {
		case r.Method == "PATCH" && r.Path == "/v1/appStoreReviewDetails/rd-1":
			body = string(r.Body)
		case r.Method == "DELETE":
			deleted = r.Path == "/v1/appStoreReviewAttachments/att-a"
		}
	}
	want := `{"data":{"type":"appStoreReviewDetails","id":"rd-1","attributes":{"demoAccountPassword":"newpass","demoAccountRequired":false,"notes":null}}}`
	if body != want || !deleted {
		t.Errorf("body=%s deleted=%v", body, deleted)
	}
}

func TestAppleReviewInfoSetDryRun(t *testing.T) {
	srv := reviewCLIServer(t)
	att := filepath.Join(t.TempDir(), "flow.pdf")
	if err := os.WriteFile(att, []byte("bytes"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "review-info", "set", "--first-name", "Grace",
		"--demo-password", "newpass", "--attachment", att, "--dry-run", "-o", "json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if strings.Contains(stdout, "newpass") || strings.Contains(stdout, "s3cret") || !strings.Contains(stdout, `"Grace"`) {
		t.Errorf("stdout = %s", stdout)
	}
	for _, r := range srv.Requests() {
		if r.Method != "GET" {
			t.Errorf("sent %s %s", r.Method, r.Path)
		}
	}
}

func TestAppleReviewInfoSetErrors(t *testing.T) {
	cases := []struct {
		name string
		args []string
		code int
		msg  string
	}{
		{"no fields", nil, 2, "no field given"},
		{"bad bool", []string{"--demo-required", "yes"}, 1, "demo_required.txt: must be true or false"},
		{"missing attachment", []string{"--attachment", "/nonexistent/x.pdf"}, 1, "not a readable file"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := reviewCLIServer(t)
			code, _, stderr := runEnv(t, buildEnv(srv), false, append([]string{"apple", "review-info", "set"}, c.args...)...)
			if code != c.code || !strings.Contains(stderr, c.msg) {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			for _, r := range srv.Requests() {
				if r.Method != "GET" {
					t.Errorf("sent %s %s", r.Method, r.Path)
				}
			}
		})
	}
}
