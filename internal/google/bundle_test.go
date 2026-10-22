package google

import (
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/testutil/playfake"
)

const (
	bundlesUpload = "/upload/androidpublisher/v3/applications/{packageName}/edits/edit-1/bundles"
	trackPath     = editPath + "/tracks/{track}"
	listingsPath  = editPath + "/listings"
)

func writeFile(t *testing.T, name string, data []byte) string {
	t.Helper()
	p := filepath.Join(t.TempDir(), name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func jsonBody(body string) playfake.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = io.WriteString(w, body)
	}
}

// bundleServer answers the upload with version code 42, listings en-US and
// de-DE, an empty current track, and echoes track updates.
func bundleServer(t *testing.T, current string) *playfake.Server {
	srv := editServer(t)
	srv.Handle("POST", bundlesUpload, playfake.JSON(http.StatusOK, "bundle.json"))
	srv.Handle("GET", listingsPath, jsonBody(`{"listings":[{"language":"en-US"},{"language":"de-DE"}]}`))
	srv.Handle("GET", trackPath, func(w http.ResponseWriter, r *http.Request) {
		jsonBody(strings.ReplaceAll(current, "TRACK", r.PathValue("track")))(w, r)
	})
	srv.Handle("PUT", trackPath, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_, _ = w.Write(b)
	})
	return srv
}

func lastBody(t *testing.T, srv *playfake.Server, method string) map[string]any {
	t.Helper()
	reqs := srv.Requests()
	for i := len(reqs) - 1; i >= 0; i-- {
		if reqs[i].Method == method {
			var m map[string]any
			if err := json.Unmarshal(reqs[i].Body, &m); err != nil {
				t.Fatal(err)
			}
			return m
		}
	}
	t.Fatalf("no %s request", method)
	return nil
}

// The file is larger than one chunk, so the upload is resumable.
func TestBundleUploadHappyPath(t *testing.T) {
	old := bundleChunkSize
	bundleChunkSize = 256 * 1024
	t.Cleanup(func() { bundleChunkSize = old })
	data := bytes.Repeat([]byte("aab-bytes-"), 100_000) // 1 MB
	srv := bundleServer(t, `{"track":"TRACK"}`)
	var got []byte
	srv.Handle("POST", bundlesUpload, func(w http.ResponseWriter, r *http.Request) {
		got, _ = io.ReadAll(r.Body)
		playfake.JSON(http.StatusOK, "bundle.json")(w, r)
	})
	var stderr strings.Builder
	res, err := newService(t, srv, &stderr).UploadBundle(t.Context(), BundleUploadOptions{
		Path: writeFile(t, "app.aab", data), Tracks: []string{"internal"}, ReleaseName: "v1",
		Status: StatusCompleted, Notes: map[string]string{"en-US": "Fixes"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, data) {
		t.Errorf("uploaded %d bytes, want %d", len(got), len(data))
	}
	resumable := false
	for _, r := range srv.Requests() {
		resumable = resumable || r.Query.Get("uploadType") == "resumable"
	}
	if !resumable {
		t.Error("upload was not resumable")
	}
	if c := strings.Join(calls(srv), ","); !strings.HasSuffix(c, "POST /edits/edit-1:commit") || strings.Contains(c, "DELETE") {
		t.Errorf("calls = %s", c)
	}
	body := lastBody(t, srv, "PUT")
	rel := body["releases"].([]any)[0].(map[string]any)
	if body["track"] != "internal" || rel["name"] != "v1" || rel["status"] != "completed" ||
		rel["versionCodes"].([]any)[0] != "42" || rel["releaseNotes"].([]any)[0].(map[string]any)["language"] != "en-US" {
		t.Errorf("track body = %v", body)
	}
	if res.VersionCode != 42 || len(res.Tracks) != 1 || res.Tracks[0].Releases[0].VersionCodes[0] != "42" {
		t.Errorf("res = %+v", res)
	}
}

func TestBundleUploadNotesDropUnlisted(t *testing.T) {
	srv := bundleServer(t, `{"track":"TRACK"}`)
	var stderr strings.Builder
	_, err := newService(t, srv, &stderr).UploadBundle(t.Context(), BundleUploadOptions{
		Path: writeFile(t, "app.aab", []byte("x")), Tracks: []string{"internal"}, Status: StatusCompleted,
		Notes: map[string]string{"en-US": "Fixes", "de-DE": "Korrekturen", "es-MX": "Arreglos", "fr-FR": "Corrections"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(stderr.String(), "es-MX") || !strings.Contains(stderr.String(), "fr-FR") || strings.Contains(stderr.String(), "en-US") {
		t.Errorf("stderr = %q", stderr.String())
	}
	rel := lastBody(t, srv, "PUT")["releases"].([]any)[0].(map[string]any)
	if n := rel["releaseNotes"].([]any); len(n) != 2 || n[0].(map[string]any)["language"] != "de-DE" {
		t.Errorf("notes = %v", n)
	}
}

func TestBundleUploadNotesTooLong(t *testing.T) {
	srv := bundleServer(t, `{"track":"TRACK"}`)
	_, err := newService(t, srv, io.Discard).UploadBundle(t.Context(), BundleUploadOptions{
		Path: writeFile(t, "app.aab", []byte("x")), Tracks: []string{"internal"}, Status: StatusCompleted,
		Notes: map[string]string{"en-US": strings.Repeat("ä", 501)},
	})
	if err == nil || clierr.ExitCode(err) != 1 || !strings.Contains(err.Error(), "en-US: 501 characters, the limit is 500") {
		t.Fatalf("err = %v", err)
	}
	if strings.Contains(strings.Join(calls(srv), ","), "bundles") {
		t.Error("uploaded despite invalid notes")
	}
	if err := ValidateReleaseNotes(map[string]string{"en-US": strings.Repeat("ä", 500)}); err != nil {
		t.Errorf("500 runes rejected: %v", err)
	}
}

// Two files for one Play language would silently overwrite each other.
func TestValidateReleaseNotesRejectsTwoLocalesForOneLanguage(t *testing.T) {
	err := ValidateReleaseNotes(map[string]string{"es-MX": "a", "es-419": "b"})
	if err == nil || clierr.ExitCode(err) != 1 || !strings.Contains(err.Error(), "release notes es-419 and es-MX are both for Play language es-419") {
		t.Fatalf("err = %v", err)
	}
}

func TestSetReleaseKeepsRules(t *testing.T) {
	const current = `{"track":"TRACK","releases":[
		{"name":"c","status":"completed","versionCodes":["1"]},
		{"name":"s","status":"inProgress","userFraction":0.1,"versionCodes":["2"]},
		{"name":"d","status":"draft","versionCodes":["3"]}]}`
	tests := []struct {
		status string
		want   string
	}{
		{StatusCompleted, "new"},
		{StatusInProgress, "c,new"},
		{StatusHalted, "c,new"},
		{StatusDraft, "c,s,new"},
	}
	for _, tt := range tests {
		t.Run(tt.status, func(t *testing.T) {
			srv := bundleServer(t, current)
			_, err := newService(t, srv, io.Discard).UploadBundle(t.Context(), BundleUploadOptions{
				Path: writeFile(t, "app.aab", []byte("x")), Tracks: []string{"beta"}, ReleaseName: "new", Status: tt.status, UserFraction: 0.5,
			})
			if err != nil {
				t.Fatal(err)
			}
			var names []string
			for _, r := range lastBody(t, srv, "PUT")["releases"].([]any) {
				names = append(names, r.(map[string]any)["name"].(string))
			}
			if got := strings.Join(names, ","); got != tt.want {
				t.Errorf("releases = %s, want %s", got, tt.want)
			}
		})
	}
}
