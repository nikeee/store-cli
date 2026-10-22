package apple

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/metadata"
	"github.com/nikeee/store-cli/internal/output"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
	"github.com/nikeee/store-cli/internal/testutil/golden"
)

// previewFiles writes n video files with different contents.
func previewFiles(t *testing.T, n int) []string {
	t.Helper()
	dir := t.TempDir()
	var paths []string
	for i := range n {
		p := filepath.Join(dir, fmt.Sprintf("%02d.mp4", i+1))
		if err := os.WriteFile(p, []byte(fmt.Sprintf("video %d", i+1)), 0o600); err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
	}
	return paths
}

// previewServer serves the preview endpoints; remote are the previews of every
// set, state is the delivery state of every preview GET.
func previewServer(t *testing.T, sets string, remote []remoteShot, state string) *ascfake.Server {
	t.Helper()
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/appStoreVersions/{id}/appStoreVersionLocalizations", ascfake.JSON(http.StatusOK, "version_localizations.json"))
	srv.Handle("GET", "/v1/appStoreVersionLocalizations/{id}/appPreviewSets", ascfake.JSON(http.StatusOK, sets))
	srv.Handle("POST", "/v1/appPreviewSets", ascfake.JSON(http.StatusCreated, "appPreviewSet_created.json"))
	srv.Handle("DELETE", "/v1/appPreviewSets/{id}", ascfake.JSON(http.StatusNoContent, "empty.json"))
	srv.Handle("DELETE", "/v1/appPreviews/{id}", ascfake.JSON(http.StatusNoContent, "empty.json"))
	srv.Handle("GET", "/v1/appPreviewSets/{id}/appPreviews", func(w http.ResponseWriter, _ *http.Request) {
		data := []any{}
		for _, r := range remote {
			data = append(data, map[string]any{"type": "appPreviews", "id": r.id,
				"attributes": map[string]any{"fileName": r.name, "sourceFileChecksum": r.sum, "assetDeliveryState": map[string]any{"state": "COMPLETE"}}})
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": data, "links": map[string]any{"self": "x"}})
	})
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
		writeJSON(w, http.StatusCreated, map[string]any{"data": map[string]any{"type": "appPreviews", "id": "pnew",
			"attributes": map[string]any{"fileName": req.Data.Attributes.FileName, "uploadOperations": []any{
				map[string]any{"method": "PUT", "url": srv.UploadURL("pnew-" + req.Data.Attributes.FileName), "offset": 0, "length": req.Data.Attributes.FileSize},
			}}}, "links": map[string]any{"self": "x"}})
	})
	srv.Handle("PATCH", "/v1/appPreviews/{id}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"type": "appPreviews", "id": r.PathValue("id")}, "links": map[string]any{"self": "x"}})
	})
	srv.Handle("GET", "/v1/appPreviews/{id}", func(w http.ResponseWriter, r *http.Request) {
		if state == "" {
			state = "COMPLETE"
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"type": "appPreviews", "id": r.PathValue("id"),
			"attributes": map[string]any{"assetDeliveryState": map[string]any{"state": state}}}, "links": map[string]any{"self": "x"}})
	})
	return srv
}

var testVersion = Version{ID: "v1", Version: "1.3.0"}

func TestUploadPreview(t *testing.T) {
	srv := previewServer(t, "appPreviewSets_empty.json", nil, "")
	files := previewFiles(t, 2)
	mov := filepath.Join(t.TempDir(), "tour.mov")
	if err := os.WriteFile(mov, []byte("quicktime"), 0o600); err != nil {
		t.Fatal(err)
	}
	files[1] = mov
	svc, _ := uploadService(t, srv)
	set, ch, err := svc.UploadPreviews(t.Context(), testVersion, PreviewUpload{Locale: "en-US", Device: iphone69, Paths: files, Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if ch.Action != metadata.ActionCreate || ch.Scope != "previews" || ch.Field != "iphone-6.9" || ch.New != "2 previews" || set.PreviewType != "IPHONE_67" {
		t.Errorf("change=%+v set=%+v", ch, set)
	}
	var created []string
	var setBody []byte
	for _, r := range srv.Requests() {
		switch {
		case r.Method == "POST" && r.Path == "/v1/appPreviewSets":
			setBody = r.Body
		case r.Method == "POST" && r.Path == "/v1/appPreviews":
			created = append(created, string(r.Body))
		}
	}
	if !strings.Contains(string(setBody), `"previewType":"IPHONE_67"`) || !strings.Contains(string(setBody), `"id":"vl-en"`) {
		t.Errorf("set body = %s", setBody)
	}
	if len(created) != 2 {
		t.Fatalf("created = %v", created)
	}
	for i, want := range []string{`"mimeType":"video/mp4"`, `"mimeType":"video/quicktime"`} {
		if !strings.Contains(created[i], want) || !strings.Contains(created[i], `"id":"pset-new"`) || strings.Contains(created[i], "previewFrameTimeCode") {
			t.Errorf("create %d = %s", i, created[i])
		}
	}
}

func TestPreviewPosterFrame(t *testing.T) {
	srv := previewServer(t, "appPreviewSets_empty.json", nil, "")
	svc, _ := uploadService(t, srv)
	paths := previewFiles(t, 1)
	_, _, err := svc.UploadPreviews(t.Context(), testVersion, PreviewUpload{Locale: "en-US", Device: iphone69, Paths: paths, PosterFrame: "00:00:05:00", Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	var patch struct {
		Data struct {
			Attributes struct {
				Uploaded             bool
				SourceFileChecksum   string
				PreviewFrameTimeCode string
			}
		}
	}
	found := false
	for _, r := range srv.Requests() {
		if r.Method == "PATCH" {
			found = true
			if err := json.Unmarshal(r.Body, &patch); err != nil {
				t.Fatal(err)
			}
		}
	}
	a := patch.Data.Attributes
	if !found || !a.Uploaded || a.PreviewFrameTimeCode != "00:00:05:00" || a.SourceFileChecksum != sums(t, paths)[0] {
		t.Errorf("commit = %+v", a)
	}
}

func TestPreviewTooMany(t *testing.T) {
	// More than 3 files are rejected before any API call.
	srv := previewServer(t, "appPreviewSets_two.json", nil, "")
	svc, _ := uploadService(t, srv)
	_, _, err := svc.UploadPreviews(t.Context(), testVersion, PreviewUpload{Locale: "en-US", Device: iphone69, Paths: previewFiles(t, 4), Replace: true})
	if err == nil || clierr.ExitCode(err) != 1 || !strings.Contains(err.Error(), "at most 3") || len(srv.Requests()) != 0 {
		t.Fatalf("err = %v, %d requests", err, len(srv.Requests()))
	}
	// Appending to a set holding 2 exceeds the maximum.
	srv = previewServer(t, "appPreviewSets_two.json", []remoteShot{{"a", "a.mp4", "s1"}, {"b", "b.mp4", "s2"}}, "")
	svc, _ = uploadService(t, srv)
	_, _, err = svc.UploadPreviews(t.Context(), testVersion, PreviewUpload{Locale: "en-US", Device: iphone69, Paths: previewFiles(t, 2)})
	if err == nil || clierr.ExitCode(err) != 1 || !strings.Contains(err.Error(), "maximum of 3") || len(mutations(srv)) != 0 {
		t.Fatalf("err = %v, mutations %v", err, mutations(srv))
	}
}

func TestPreviewFileTypes(t *testing.T) {
	dir := t.TempDir()
	bad := filepath.Join(dir, "clip.avi")
	if err := os.WriteFile(bad, []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	srv := previewServer(t, "appPreviewSets_empty.json", nil, "")
	svc, _ := uploadService(t, srv)
	_, _, err := svc.UploadPreviews(t.Context(), testVersion, PreviewUpload{Locale: "en-US", Device: iphone69, Paths: append(previewFiles(t, 1), bad)})
	if err == nil || clierr.ExitCode(err) != 1 || !strings.Contains(err.Error(), "clip.avi") || len(srv.Requests()) != 0 {
		t.Fatalf("err = %v, %d requests", err, len(srv.Requests()))
	}
	for ext, want := range map[string]string{".mp4": "video/mp4", ".MOV": "video/quicktime", ".m4v": "video/x-m4v"} {
		if got, err := previewMimeType("a" + ext); err != nil || got != want {
			t.Errorf("%s: %q %v", ext, got, err)
		}
	}
}

func TestPreviewReplaceSkipsEqual(t *testing.T) {
	paths := previewFiles(t, 2)
	sm := sums(t, paths)
	srv := previewServer(t, "appPreviewSets_two.json", []remoteShot{{"a", "01.mp4", sm[0]}, {"b", "02.mp4", sm[1]}}, "")
	svc, _ := uploadService(t, srv)
	_, ch, err := svc.UploadPreviews(t.Context(), testVersion, PreviewUpload{Locale: "en-US", Device: iphone69, Paths: paths, Replace: true})
	if err != nil || ch.Action != metadata.ActionSkip || len(mutations(srv)) != 0 {
		t.Fatalf("change=%+v err=%v mutations=%v", ch, err, mutations(srv))
	}
	if q := srv.Requests()[1].Query; q.Get("filter[previewType]") != "IPHONE_67" {
		t.Errorf("query = %s", q.Encode())
	}

	// Different order replaces: old previews are deleted first.
	srv = previewServer(t, "appPreviewSets_two.json", []remoteShot{{"a", "02.mp4", sm[1]}, {"b", "01.mp4", sm[0]}}, "")
	svc, _ = uploadService(t, srv)
	_, ch, err = svc.UploadPreviews(t.Context(), testVersion, PreviewUpload{Locale: "en-US", Device: iphone69, Paths: paths, Replace: true, Timeout: time.Minute})
	if err != nil || ch.Action != metadata.ActionReplace {
		t.Fatalf("change=%+v err=%v", ch, err)
	}
	if m := mutations(srv); len(m) < 2 || m[0] != "DELETE /v1/appPreviews/a" || m[1] != "DELETE /v1/appPreviews/b" {
		t.Errorf("mutations = %v", m)
	}
}

func TestListPreviews(t *testing.T) {
	srv := previewServer(t, "appPreviewSets_two.json", []remoteShot{{"a", "01.mp4", "s1"}}, "")
	svc, _ := uploadService(t, srv)
	sets, err := svc.ListPreviews(t.Context(), testVersion, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 4 || sets[0].Locale != "de-DE" || sets[0].Device != "iphone-6.9" || sets[1].Device != "ipad-13" || sets[0].PreviewType != "IPHONE_67" {
		t.Fatalf("sets = %+v", sets)
	}
	b, _ := json.MarshalIndent(sets[:1], "", "  ")
	golden.Assert(t, "preview_set.json", b)

	var sb strings.Builder
	if err := sets[:2].RenderText(&sb, output.NewStyles(false)); err != nil {
		t.Fatal(err)
	}
	golden.Assert(t, "preview_sets.text", []byte(sb.String()))
}

func TestDeletePreviews(t *testing.T) {
	srv := previewServer(t, "appPreviewSets_two.json", []remoteShot{{"a", "01.mp4", "s1"}}, "")
	svc, _ := uploadService(t, srv)
	res, ch, err := svc.DeletePreviews(t.Context(), testVersion, "en-US", iphone69, true)
	if err != nil || res.Deleted || ch.Action != metadata.ActionDelete || ch.Old != "1 preview" || len(mutations(srv)) != 0 {
		t.Fatalf("dry run: res=%+v ch=%+v err=%v", res, ch, err)
	}
	res, _, err = svc.DeletePreviews(t.Context(), testVersion, "en-US", iphone69, false)
	if err != nil || !res.Deleted {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if m := mutations(srv); len(m) != 1 || m[0] != "DELETE /v1/appPreviewSets/pset-1" {
		t.Errorf("mutations = %v", m)
	}
}

func TestPreviewWatchRejected(t *testing.T) {
	for _, alias := range []string{"watch-10", "watch-ultra", "watch-3"} {
		if _, err := DeviceForPreviews(alias); err == nil || clierr.ExitCode(err) != 2 {
			t.Errorf("%s: err = %v", alias, err)
		}
	}
	if _, err := DeviceForPreviews("iphone-9"); err == nil || clierr.ExitCode(err) != 2 {
		t.Errorf("unknown: %v", err)
	}
	if d, err := DeviceForPreviews("iphone-6.9"); err != nil || d.PreviewType != "IPHONE_67" {
		t.Errorf("iphone: %+v %v", d, err)
	}
}
