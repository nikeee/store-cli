package cli_test

import (
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/testutil/ascfake"
	"github.com/nikeee/store-cli/internal/testutil/golden"
)

// previewCLIServer serves a version with two localizations, the sets of
// appPreviewSets_two.json and the previews of appPreviews_two.json.
func previewCLIServer(t *testing.T) *ascfake.Server {
	t.Helper()
	srv := versionCLIServer(t, "app_appStoreVersions.json")
	srv.Handle("GET", "/v1/appStoreVersions/{id}/appStoreVersionLocalizations", ascfake.JSON(http.StatusOK, "version_localizations.json"))
	srv.Handle("GET", "/v1/appStoreVersionLocalizations/{id}/appPreviewSets", ascfake.JSON(http.StatusOK, "appPreviewSets_two.json"))
	srv.Handle("GET", "/v1/appPreviewSets/{id}/appPreviews", ascfake.JSON(http.StatusOK, "appPreviews_two.json"))
	srv.Handle("DELETE", "/v1/appPreviewSets/{id}", ascfake.JSON(http.StatusNoContent, "empty.json"))
	srv.Handle("DELETE", "/v1/appPreviews/{id}", ascfake.JSON(http.StatusNoContent, "empty.json"))
	srv.Handle("POST", "/v1/appPreviews", func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var req struct {
			Data struct {
				Attributes struct {
					FileName string `json:"fileName"`
					FileSize int    `json:"fileSize"`
				} `json:"attributes"`
			} `json:"data"`
		}
		_ = json.Unmarshal(body, &req)
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusCreated)
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"type": "appPreviews", "id": "new-1",
			"attributes": map[string]any{"fileName": req.Data.Attributes.FileName, "uploadOperations": []any{
				map[string]any{"method": "PUT", "url": srv.UploadURL("new-1"), "offset": 0, "length": req.Data.Attributes.FileSize},
			}}}, "links": map[string]any{"self": "x"}})
	})
	srv.Handle("PATCH", "/v1/appPreviews/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"type":"appPreviews","id":"new-1"},"links":{"self":"x"}}`)
	})
	srv.Handle("GET", "/v1/appPreviews/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"type":"appPreviews","id":"new-1","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}},"links":{"self":"x"}}`)
	})
	return srv
}

func videoArg(t *testing.T, name string) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, []byte("video"), 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func TestApplePreviewsListGolden(t *testing.T) {
	for _, mode := range []string{"text", "json", "toon"} {
		t.Run(mode, func(t *testing.T) {
			srv := previewCLIServer(t)
			code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "previews", "list", "-o", mode)
			if code != 0 || stderr != "" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			golden.Assert(t, "apple_previews_list."+mode, []byte(stdout))
		})
	}
}

func TestApplePreviewsUpload(t *testing.T) {
	srv := previewCLIServer(t)
	file := videoArg(t, "intro.mp4")
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "previews", "upload", "--locale", "en-US", "--device", "iphone-6.9", "--poster-frame", "00:00:03:00", file, "-o", "json")
	if code != 0 || !strings.Contains(stdout, `"previewType": "IPHONE_67"`) || !strings.Contains(stderr, "uploading preview intro.mp4") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	var patch string
	for _, r := range srv.Requests() {
		if r.Method == "PATCH" {
			patch = string(r.Body)
		}
	}
	if !strings.Contains(patch, `"previewFrameTimeCode":"00:00:03:00"`) {
		t.Errorf("commit = %s", patch)
	}
}

func TestPreviewNeedsDevice(t *testing.T) {
	file := videoArg(t, "intro.mp4")
	for name, args := range map[string][]string{
		"upload no device":   {"upload", "--locale", "en-US", file},
		"delete no device":   {"delete", "--locale", "en-US"},
		"delete no locale":   {"delete", "--device", "iphone-6.9"},
		"upload no locale":   {"upload", "--device", "iphone-6.9", file},
		"upload no files":    {"upload", "--locale", "en-US", "--device", "iphone-6.9"},
		"upload watch":       {"upload", "--locale", "en-US", "--device", "watch-10", file},
		"delete watch":       {"delete", "--locale", "en-US", "--device", "watch-10"},
		"list watch":         {"list", "--device", "watch-10"},
		"upload unknown dev": {"upload", "--locale", "en-US", "--device", "iphone-9", file},
		"upload bad timeout": {"upload", "--locale", "en-US", "--device", "iphone-6.9", "--timeout", "0s", file},
	} {
		t.Run(name, func(t *testing.T) {
			srv := previewCLIServer(t)
			code, _, stderr := runEnv(t, buildEnv(srv), false, append([]string{"apple", "previews"}, args...)...)
			if code != 2 || stderr == "" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			if n := len(srv.Requests()); n != 0 {
				t.Errorf("%d API requests, want 0", n)
			}
		})
	}
}

func TestApplePreviewsUploadLocalFailuresSendNothing(t *testing.T) {
	srv := previewCLIServer(t)
	code, _, stderr := runEnv(t, buildEnv(srv), false, "apple", "previews", "upload", "--locale", "en-US", "--device", "iphone-6.9", videoArg(t, "clip.avi"))
	if code != 1 || !strings.Contains(stderr, "clip.avi") || len(srv.Requests()) != 0 {
		t.Fatalf("code=%d stderr=%q requests=%d", code, stderr, len(srv.Requests()))
	}
}

func TestApplePreviewsDelete(t *testing.T) {
	srv := previewCLIServer(t)
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "previews", "delete", "--locale", "en-US", "--device", "iphone-6.9", "-o", "json")
	if code != 0 || strings.Join(strings.Fields(stdout), "") != `{"locale":"en-US","device":"iphone-6.9","deleted":true}` {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	reqs := srv.Requests()
	if last := reqs[len(reqs)-1]; last.Method != "DELETE" || last.Path != "/v1/appPreviewSets/pset-1" {
		t.Errorf("last request = %s %s", last.Method, last.Path)
	}
}

func TestApplePreviewsMutationsDryRun(t *testing.T) {
	file := videoArg(t, "intro.mp4")
	for _, args := range [][]string{
		{"previews", "upload", "--locale", "en-US", "--device", "iphone-6.9", file},
		{"previews", "upload", "--locale", "en-US", "--device", "iphone-6.9", "--replace", file},
		{"previews", "delete", "--locale", "en-US", "--device", "iphone-6.9"},
	} {
		srv := previewCLIServer(t)
		code, stdout, stderr := runEnv(t, buildEnv(srv), false, append(append([]string{"apple"}, args...), "--dry-run", "-o", "json")...)
		if code != 0 || !strings.Contains(stdout, `"changes"`) || !strings.Contains(stdout, `"previews"`) {
			t.Fatalf("%v: code=%d stdout=%q stderr=%q", args, code, stdout, stderr)
		}
		for _, r := range srv.Requests() {
			if r.Method != http.MethodGet {
				t.Errorf("%v: dry run sent %s %s", args, r.Method, r.Path)
			}
		}
	}
}
