package cli_test

import (
	"encoding/json"
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/testutil/ascfake"
	"github.com/nikeee/store-cli/internal/testutil/golden"
	"github.com/nikeee/store-cli/internal/testutil/pngfake"
)

// screenshotCLIServer serves a version with two localizations, the sets of
// appScreenshotSets_two.json and the screenshots of appScreenshots_two.json.
func screenshotCLIServer(t *testing.T) *ascfake.Server {
	t.Helper()
	srv := versionCLIServer(t, "app_appStoreVersions.json")
	srv.Handle("GET", "/v1/appStoreVersions/{id}/appStoreVersionLocalizations", ascfake.JSON(http.StatusOK, "version_localizations.json"))
	srv.Handle("GET", "/v1/appStoreVersionLocalizations/{id}/appScreenshotSets", ascfake.JSON(http.StatusOK, "appScreenshotSets_two.json"))
	srv.Handle("GET", "/v1/appScreenshotSets/{id}/appScreenshots", ascfake.JSON(http.StatusOK, "appScreenshots_two.json"))
	srv.Handle("DELETE", "/v1/appScreenshotSets/{id}", ascfake.JSON(http.StatusNoContent, "empty.json"))
	srv.Handle("DELETE", "/v1/appScreenshots/{id}", ascfake.JSON(http.StatusNoContent, "empty.json"))
	srv.Handle("POST", "/v1/appScreenshots", func(w http.ResponseWriter, r *http.Request) {
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
		_ = json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"type": "appScreenshots", "id": "new-1",
			"attributes": map[string]any{"fileName": req.Data.Attributes.FileName, "uploadOperations": []any{
				map[string]any{"method": "PUT", "url": srv.UploadURL("new-1"), "offset": 0, "length": req.Data.Attributes.FileSize},
			}}}, "links": map[string]any{"self": "x"}})
	})
	srv.Handle("PATCH", "/v1/appScreenshots/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"type":"appScreenshots","id":"new-1"},"links":{"self":"x"}}`)
	})
	srv.Handle("GET", "/v1/appScreenshots/{id}", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, `{"data":{"type":"appScreenshots","id":"new-1","attributes":{"assetDeliveryState":{"state":"COMPLETE"}}},"links":{"self":"x"}}`)
	})
	return srv
}

func shotArgs(t *testing.T, w, h int) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), "01.png")
	pngfake.Write(t, p, w, h, 7)
	return p
}

func TestAppleScreenshotsListGolden(t *testing.T) {
	for _, mode := range []string{"text", "json", "toon"} {
		t.Run(mode, func(t *testing.T) {
			srv := screenshotCLIServer(t)
			code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "screenshots", "list", "-o", mode)
			if code != 0 || stderr != "" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			golden.Assert(t, "apple_screenshots_list."+mode, []byte(stdout))
		})
	}
}

func TestAppleScreenshotsListFilters(t *testing.T) {
	srv := screenshotCLIServer(t)
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "screenshots", "list", "--locale", "de-DE", "--device", "iphone-6.9", "-o", "json")
	if code != 0 || strings.Count(stdout, `"locale"`) != 1 || !strings.Contains(stdout, "de-DE") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	code, _, stderr = runEnv(t, buildEnv(srv), false, "apple", "screenshots", "list", "--device", "iphone-9")
	if code != 2 || !strings.Contains(stderr, "iphone-9") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	code, _, stderr = runEnv(t, buildEnv(srv), false, "apple", "screenshots", "list", "--locale", "ja")
	if code != 1 || !strings.Contains(stderr, "store apple listing locale add ja") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestAppleScreenshotsUpload(t *testing.T) {
	srv := screenshotCLIServer(t)
	file := shotArgs(t, 1290, 2796)
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "screenshots", "upload", "--locale", "en-US", file, "-o", "json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	// Device detected from the pixel size; the set of the fixture is reused and appended to.
	if !strings.Contains(stdout, `"device": "iphone-6.9"`) || !strings.Contains(stderr, "uploading screenshot 01.png") {
		t.Errorf("stdout=%q stderr=%q", stdout, stderr)
	}
}

func TestAppleScreenshotsUploadLocalFailuresSendNothing(t *testing.T) {
	alpha := filepath.Join(t.TempDir(), "alpha.png")
	pngfake.WriteAlpha(t, alpha, 1290, 2796)
	odd := shotArgs(t, 123, 456)
	for name, args := range map[string][]string{
		"alpha":        {"--locale", "en-US", "--device", "iphone-6.9", alpha},
		"unknown size": {"--locale", "en-US", odd},
		"wrong device": {"--locale", "en-US", "--device", "iphone-6.5", shotArgs(t, 1290, 2796)},
	} {
		t.Run(name, func(t *testing.T) {
			srv := screenshotCLIServer(t)
			code, _, stderr := runEnv(t, buildEnv(srv), false, append([]string{"apple", "screenshots", "upload"}, args...)...)
			if code != 1 || stderr == "" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			if name == "unknown size" && !strings.Contains(stderr, "accepted sizes") {
				t.Errorf("stderr = %q", stderr)
			}
			if n := len(srv.Requests()); n != 0 {
				t.Errorf("%d API requests, want 0", n)
			}
		})
	}
}

func TestAppleScreenshotsUsageErrors(t *testing.T) {
	file := shotArgs(t, 1290, 2796)
	for name, args := range map[string][]string{
		"no files":       {"upload", "--locale", "en-US"},
		"no locale":      {"upload", file},
		"bad timeout":    {"upload", "--locale", "en-US", "--timeout", "0s", file},
		"unknown device": {"upload", "--locale", "en-US", "--device", "iphone-9", file},
		"delete device":  {"delete", "--locale", "en-US"},
		"delete locale":  {"delete", "--device", "iphone-6.9"},
	} {
		t.Run(name, func(t *testing.T) {
			srv := screenshotCLIServer(t)
			code, _, stderr := runEnv(t, buildEnv(srv), false, append([]string{"apple", "screenshots"}, args...)...)
			if code != 2 || stderr == "" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			if n := len(srv.Requests()); n != 0 {
				t.Errorf("%d API requests, want 0", n)
			}
		})
	}
}

func TestAppleScreenshotsDelete(t *testing.T) {
	srv := screenshotCLIServer(t)
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "screenshots", "delete", "--locale", "en-US", "--device", "iphone-6.9", "-o", "json")
	if code != 0 || strings.Join(strings.Fields(stdout), "") != `{"locale":"en-US","device":"iphone-6.9","action":"delete"}` {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	reqs := srv.Requests()
	if last := reqs[len(reqs)-1]; last.Method != "DELETE" || last.Path != "/v1/appScreenshotSets/set-1" {
		t.Errorf("last request = %s %s", last.Method, last.Path)
	}
}

// Nothing to delete is "nothing to do": exit 0 with action skip.
func TestAppleMediaDeleteMissingSkips(t *testing.T) {
	for _, kind := range []string{"screenshots", "previews"} {
		srv := screenshotCLIServer(t)
		if kind == "previews" {
			srv = previewCLIServer(t)
		}
		code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", kind, "delete", "--locale", "fr-FR", "--device", "iphone-6.9", "-o", "json")
		if code != 0 || strings.Join(strings.Fields(stdout), "") != `{"locale":"fr-FR","device":"iphone-6.9","action":"skip"}` {
			t.Fatalf("%s: code=%d stdout=%q stderr=%q", kind, code, stdout, stderr)
		}
		for _, r := range srv.Requests() {
			if r.Method != http.MethodGet {
				t.Errorf("%s: sent %s %s", kind, r.Method, r.Path)
			}
		}
	}
}

func TestAppleScreenshotsMutationsDryRun(t *testing.T) {
	file := shotArgs(t, 1290, 2796)
	for _, args := range [][]string{
		{"screenshots", "upload", "--locale", "en-US", file},
		{"screenshots", "upload", "--locale", "en-US", "--replace", file},
		{"screenshots", "delete", "--locale", "en-US", "--device", "iphone-6.9"},
	} {
		srv := screenshotCLIServer(t)
		code, stdout, stderr := runEnv(t, buildEnv(srv), false, append(append([]string{"apple"}, args...), "--dry-run", "-o", "json")...)
		if code != 0 || !strings.Contains(stdout, `"changes"`) || !strings.Contains(stdout, `"screenshots"`) {
			t.Fatalf("%v: code=%d stdout=%q stderr=%q", args, code, stdout, stderr)
		}
		for _, r := range srv.Requests() {
			if r.Method != http.MethodGet {
				t.Errorf("%v: dry run sent %s %s", args, r.Method, r.Path)
			}
		}
	}
}
