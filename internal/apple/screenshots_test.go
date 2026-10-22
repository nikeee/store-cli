package apple

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/media"
	"github.com/nikeee/store-cli/internal/metadata"
	"github.com/nikeee/store-cli/internal/output"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
	"github.com/nikeee/store-cli/internal/testutil/golden"
	"github.com/nikeee/store-cli/internal/testutil/pngfake"
)

var iphone69 = func() media.Device { d, _ := media.DeviceByAlias("iphone-6.9"); return d }()

// shotFiles writes n valid iphone-6.9 screenshots with different contents.
func shotFiles(t *testing.T, n int) []string {
	t.Helper()
	dir := t.TempDir()
	var paths []string
	for i := range n {
		p := filepath.Join(dir, fmt.Sprintf("%02d.png", i+1))
		pngfake.Write(t, p, 1290, 2796, uint8(i+1))
		paths = append(paths, p)
	}
	return paths
}

func sums(t *testing.T, paths []string) []string {
	t.Helper()
	l, err := media.MD5List(paths)
	if err != nil {
		t.Fatal(err)
	}
	return l
}

type remoteShot struct{ id, name, sum string }

// shotServer serves the screenshot endpoints. sets is the fixture answering the
// set list, remote the screenshots of every set, state the delivery state of a
// screenshot on its n-th (1-based) GET.
type shotServer struct {
	*ascfake.Server
	mu     sync.Mutex
	nextID int
	polls  map[string]int
	// remoteState is the delivery state of listed screenshots (default COMPLETE).
	remoteState string
}

func newShotServer(t *testing.T, sets string, remote []remoteShot, state func(n int) (string, []map[string]string)) *shotServer {
	t.Helper()
	ss := &shotServer{Server: ascfake.New(t), polls: map[string]int{}}
	srv := ss.Server
	srv.Handle("GET", "/v1/appStoreVersions/{id}/appStoreVersionLocalizations", ascfake.JSON(http.StatusOK, "version_localizations.json"))
	srv.Handle("GET", "/v1/appStoreVersionLocalizations/{id}/appScreenshotSets", ascfake.JSON(http.StatusOK, sets))
	srv.Handle("POST", "/v1/appScreenshotSets", ascfake.JSON(http.StatusCreated, "appScreenshotSet_created.json"))
	srv.Handle("DELETE", "/v1/appScreenshotSets/{id}", ascfake.JSON(http.StatusNoContent, "empty.json"))
	srv.Handle("DELETE", "/v1/appScreenshots/{id}", ascfake.JSON(http.StatusNoContent, "empty.json"))
	srv.Handle("GET", "/v1/appScreenshotSets/{id}/appScreenshots", func(w http.ResponseWriter, _ *http.Request) {
		data := []any{}
		for _, r := range remote {
			data = append(data, map[string]any{"type": "appScreenshots", "id": r.id,
				"attributes": map[string]any{"fileName": r.name, "sourceFileChecksum": r.sum, "assetDeliveryState": map[string]any{"state": ss.state()}}})
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": data, "links": map[string]any{"self": "x"}})
	})
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
		ss.mu.Lock()
		ss.nextID++
		id := fmt.Sprintf("new-%d", ss.nextID)
		ss.mu.Unlock()
		writeJSON(w, http.StatusCreated, map[string]any{"data": map[string]any{"type": "appScreenshots", "id": id,
			"attributes": map[string]any{
				"fileName": req.Data.Attributes.FileName,
				"uploadOperations": []any{map[string]any{"method": "PUT", "url": srv.UploadURL(id), "offset": 0, "length": req.Data.Attributes.FileSize,
					"requestHeaders": []any{map[string]any{"name": "Content-Type", "value": "application/octet-stream"}}}},
			}}, "links": map[string]any{"self": "x"}})
	})
	srv.Handle("PATCH", "/v1/appScreenshots/{id}", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"type": "appScreenshots", "id": r.PathValue("id")}, "links": map[string]any{"self": "x"}})
	})
	srv.Handle("GET", "/v1/appScreenshots/{id}", func(w http.ResponseWriter, r *http.Request) {
		id := r.PathValue("id")
		ss.mu.Lock()
		ss.polls[id]++
		n := ss.polls[id]
		ss.mu.Unlock()
		st, errs := state(n)
		ds := map[string]any{"state": st}
		if errs != nil {
			ds["errors"] = errs
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"type": "appScreenshots", "id": id,
			"attributes": map[string]any{"assetDeliveryState": ds}}, "links": map[string]any{"self": "x"}})
	})
	return ss
}

func (ss *shotServer) state() string {
	if ss.remoteState == "" {
		return "COMPLETE"
	}
	return ss.remoteState
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func instantly(int) (string, []map[string]string) { return "COMPLETE", nil }

// mutations returns the non-GET API requests (without presigned uploads).
func mutations(srv *ascfake.Server) []string {
	var out []string
	for _, r := range srv.Requests() {
		if r.Method != http.MethodGet && !strings.HasPrefix(r.Path, "/upload/") {
			out = append(out, r.Method+" "+r.Path)
		}
	}
	return out
}

func apiCalls(srv *ascfake.Server) []string {
	var out []string
	for _, r := range srv.Requests() {
		if !strings.HasPrefix(r.Path, "/upload/") {
			out = append(out, r.Method+" "+r.Path)
		}
	}
	return out
}

func TestUploadScreenshotsHappyPath(t *testing.T) {
	srv := newShotServer(t, "appScreenshotSets_empty.json", nil, instantly)
	paths := shotFiles(t, 2)
	svc, _ := uploadService(t, srv.Server)
	ch, err := svc.SyncScreenshots(t.Context(), "vl-en", "en-US", iphone69, paths, false)
	if err != nil {
		t.Fatal(err)
	}
	if ch.Action != metadata.ActionCreate || ch.Scope != "screenshots" || ch.Locale != "en-US" || ch.Field != "iphone-6.9" || ch.New != "2 screenshots" {
		t.Errorf("change = %+v", ch)
	}
	want := []string{
		"GET /v1/appStoreVersionLocalizations/vl-en/appScreenshotSets",
		"POST /v1/appScreenshotSets",
		"POST /v1/appScreenshots", "PATCH /v1/appScreenshots/new-1", "GET /v1/appScreenshots/new-1",
		"POST /v1/appScreenshots", "PATCH /v1/appScreenshots/new-2", "GET /v1/appScreenshots/new-2",
	}
	if got := apiCalls(srv.Server); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("requests:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}

	var posts, patches [][]byte
	var uploads []ascfake.Request
	for _, r := range srv.Requests() {
		switch {
		case strings.HasPrefix(r.Path, "/upload/"):
			uploads = append(uploads, r)
		case r.Method == "POST" && r.Path == "/v1/appScreenshots":
			posts = append(posts, r.Body)
		case r.Method == "PATCH":
			patches = append(patches, r.Body)
		}
	}
	for i, p := range paths {
		content, _ := os.ReadFile(p)
		var create struct {
			Data struct {
				Attributes struct {
					FileName string
					FileSize int
				}
				Relationships struct {
					Set struct{ Data struct{ Type, ID string } } `json:"appScreenshotSet"`
				}
			}
		}
		if err := json.Unmarshal(posts[i], &create); err != nil {
			t.Fatal(err)
		}
		a := create.Data.Attributes
		if a.FileName != filepath.Base(p) || a.FileSize != len(content) || create.Data.Relationships.Set.Data.ID != "set-new" || create.Data.Relationships.Set.Data.Type != "appScreenshotSets" {
			t.Errorf("create %d = %s", i, posts[i])
		}
		var patch struct {
			Data struct {
				ID         string
				Attributes struct {
					Uploaded           bool
					SourceFileChecksum string
				}
			}
		}
		if err := json.Unmarshal(patches[i], &patch); err != nil {
			t.Fatal(err)
		}
		if !patch.Data.Attributes.Uploaded || patch.Data.Attributes.SourceFileChecksum != sums(t, paths)[i] || patch.Data.ID != fmt.Sprintf("new-%d", i+1) {
			t.Errorf("commit %d = %s", i, patches[i])
		}
		if string(uploads[i].Body) != string(content) {
			t.Errorf("upload %d: %d bytes, want %d", i, len(uploads[i].Body), len(content))
		}
	}
}

func TestSyncScreenshotsUnchangedNoMutation(t *testing.T) {
	paths := shotFiles(t, 2)
	sm := sums(t, paths)
	srv := newShotServer(t, "appScreenshotSets_two.json", []remoteShot{{"a", "01.png", sm[0]}, {"b", "02.png", sm[1]}}, instantly)
	svc, _ := uploadService(t, srv.Server)
	ch, err := svc.SyncScreenshots(t.Context(), "vl-en", "en-US", iphone69, paths, false)
	if err != nil {
		t.Fatal(err)
	}
	if ch.Action != metadata.ActionSkip || ch.Old != "2 screenshots" || ch.New != "2 screenshots" {
		t.Errorf("change = %+v", ch)
	}
	if m := mutations(srv.Server); len(m) != 0 {
		t.Errorf("mutations: %v", m)
	}
	q := srv.Requests()[0].Query
	if q.Get("filter[screenshotDisplayType]") != "APP_IPHONE_67" {
		t.Errorf("query = %s", q.Encode())
	}
}

func TestSyncScreenshotsReplaces(t *testing.T) {
	paths := shotFiles(t, 2)
	sm := sums(t, paths)
	// Same files, different order: not equal.
	srv := newShotServer(t, "appScreenshotSets_two.json", []remoteShot{{"a", "02.png", sm[1]}, {"b", "01.png", sm[0]}, {"c", "03.png", "ccc"}}, instantly)
	svc, _ := uploadService(t, srv.Server)
	ch, err := svc.SyncScreenshots(t.Context(), "vl-en", "en-US", iphone69, paths, false)
	if err != nil {
		t.Fatal(err)
	}
	if ch.Action != metadata.ActionReplace || ch.Old != "3 screenshots" || ch.New != "2 screenshots" {
		t.Errorf("change = %+v", ch)
	}
	want := []string{
		"DELETE /v1/appScreenshots/a", "DELETE /v1/appScreenshots/b", "DELETE /v1/appScreenshots/c",
		"POST /v1/appScreenshots", "PATCH /v1/appScreenshots/new-1", "POST /v1/appScreenshots", "PATCH /v1/appScreenshots/new-2",
	}
	if got := mutations(srv.Server); strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("mutations:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

func TestSyncScreenshotsDryRun(t *testing.T) {
	paths := shotFiles(t, 1)
	for _, sets := range []string{"appScreenshotSets_empty.json", "appScreenshotSets_two.json"} {
		srv := newShotServer(t, sets, []remoteShot{{"a", "x.png", "other"}}, instantly)
		svc, _ := uploadService(t, srv.Server)
		ch, err := svc.SyncScreenshots(t.Context(), "vl-en", "en-US", iphone69, paths, true)
		if err != nil || ch.Action == "" || ch.Action == metadata.ActionSkip {
			t.Fatalf("%s: change=%+v err=%v", sets, ch, err)
		}
		if m := mutations(srv.Server); len(m) != 0 {
			t.Errorf("%s: dry run mutated: %v", sets, m)
		}
	}
}

func TestScreenshotAlphaRejectedLocally(t *testing.T) {
	srv := newShotServer(t, "appScreenshotSets_empty.json", nil, instantly)
	paths := shotFiles(t, 1)
	alpha := filepath.Join(t.TempDir(), "alpha.png")
	pngfake.WriteAlpha(t, alpha, 1290, 2796)
	small := filepath.Join(t.TempDir(), "small.png")
	pngfake.Write(t, small, 100, 100, 1)
	svc, _ := uploadService(t, srv.Server)
	for name, bad := range map[string]string{"alpha": alpha, "size": small} {
		_, err := svc.SyncScreenshots(t.Context(), "vl-en", "en-US", iphone69, append(paths, bad), false)
		if err == nil || clierr.ExitCode(err) != 1 || !strings.Contains(err.Error(), filepath.Base(bad)) {
			t.Errorf("%s: err = %v (exit %d)", name, err, clierr.ExitCode(err))
		}
	}
	if n := len(srv.Requests()); n != 0 {
		t.Errorf("%d API requests, want 0", n)
	}
}

func TestUploadScreenshotSlowProcessing(t *testing.T) {
	srv := newShotServer(t, "appScreenshotSets_two.json", nil, func(n int) (string, []map[string]string) {
		if n <= 5 {
			return "UPLOAD_COMPLETE", nil
		}
		return "COMPLETE", nil
	})
	svc, stderr := uploadService(t, srv.Server)
	if _, err := svc.SyncScreenshots(t.Context(), "vl-en", "en-US", iphone69, shotFiles(t, 1), false); err != nil {
		t.Fatal(err)
	}
	polls := 0
	for _, r := range srv.Requests() {
		if r.Method == "GET" && r.Path == "/v1/appScreenshots/new-1" {
			polls++
		}
	}
	if polls != 6 {
		t.Errorf("%d polls, want 6", polls)
	}
	out := stderr.String()
	if strings.Count(out, "UPLOAD_COMPLETE") != 5 || !strings.Contains(out, "01.png") || !strings.Contains(out, "4m elapsed") {
		t.Errorf("progress:\n%s", out)
	}
}

func TestUploadScreenshotTimeout(t *testing.T) {
	srv := newShotServer(t, "appScreenshotSets_two.json", nil, func(int) (string, []map[string]string) { return "UPLOAD_COMPLETE", nil })
	svc, _ := uploadService(t, srv.Server) // each poll advances the clock by 1m
	_, _, err := svc.syncMedia(t.Context(), screenshotMedia, mediaSync{
		localizationID: "vl-en",
		mediaUpload:    mediaUpload{locale: "en-US", device: iphone69, paths: shotFiles(t, 1), replace: true, timeout: 3 * time.Minute, asset: screenshotKind},
	})
	if err == nil || clierr.ExitCode(err) != 1 {
		t.Fatalf("err = %v", err)
	}
	for _, want := range []string{"timed out after 3m0s", "screenshot 01.png", "UPLOAD_COMPLETE"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error %q lacks %q", err, want)
		}
	}
}

func TestUploadScreenshotFailed(t *testing.T) {
	srv := newShotServer(t, "appScreenshotSets_two.json", nil, func(int) (string, []map[string]string) {
		return "FAILED", []map[string]string{{"code": "IMAGE_TOO_SMALL", "description": "The image is too small."}}
	})
	svc, _ := uploadService(t, srv.Server)
	_, err := svc.SyncScreenshots(t.Context(), "vl-en", "en-US", iphone69, shotFiles(t, 1), false)
	if err == nil || clierr.ExitCode(err) != 1 || !strings.Contains(err.Error(), "IMAGE_TOO_SMALL The image is too small.") || !strings.Contains(err.Error(), "01.png") {
		t.Fatalf("err = %v", err)
	}
}

func TestUploadScreenshotPartFails(t *testing.T) {
	srv := newShotServer(t, "appScreenshotSets_two.json", nil, instantly)
	srv.Handle("PUT", "/upload/{name}", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) })
	svc, _ := uploadService(t, srv.Server)
	_, err := svc.SyncScreenshots(t.Context(), "vl-en", "en-US", iphone69, shotFiles(t, 1), false)
	if err == nil || !strings.Contains(err.Error(), "upload screenshot 01.png") || strings.Contains(err.Error(), "fake-signature") {
		t.Fatalf("err = %v", err)
	}
	for _, m := range mutations(srv.Server) {
		if strings.HasPrefix(m, "PATCH") {
			t.Errorf("committed a failed upload: %s", m)
		}
	}
}

func TestTooManyScreenshots(t *testing.T) {
	// Append to a set holding 9: two more are too many; nothing is uploaded.
	var remote []remoteShot
	for i := range 9 {
		remote = append(remote, remoteShot{fmt.Sprintf("r%d", i), "r.png", fmt.Sprintf("sum%d", i)})
	}
	srv := newShotServer(t, "appScreenshotSets_two.json", remote, instantly)
	svc, _ := uploadService(t, srv.Server)
	v := Version{ID: "v1", Version: "1.3.0"}
	_, _, err := svc.UploadScreenshots(t.Context(), v, ScreenshotUpload{Locale: "en-US", Device: iphone69, Paths: shotFiles(t, 2), Timeout: time.Minute})
	if err == nil || clierr.ExitCode(err) != 1 || !strings.Contains(err.Error(), "maximum of 10") {
		t.Fatalf("err = %v", err)
	}
	if m := mutations(srv.Server); len(m) != 0 {
		t.Errorf("mutations: %v", m)
	}
	// More than 10 files are rejected before any API call.
	srv2 := newShotServer(t, "appScreenshotSets_two.json", nil, instantly)
	svc2, _ := uploadService(t, srv2.Server)
	_, err = svc2.SyncScreenshots(t.Context(), "vl-en", "en-US", iphone69, shotFiles(t, 11), false)
	if err == nil || !strings.Contains(err.Error(), "at most 10") || len(srv2.Requests()) != 0 {
		t.Fatalf("err = %v, %d requests", err, len(srv2.Requests()))
	}
}

func TestUploadScreenshotsAppends(t *testing.T) {
	srv := newShotServer(t, "appScreenshotSets_two.json", []remoteShot{{"a", "old.png", "oldsum"}}, instantly)
	svc, _ := uploadService(t, srv.Server)
	v := Version{ID: "v1", Version: "1.3.0"}
	set, ch, err := svc.UploadScreenshots(t.Context(), v, ScreenshotUpload{Locale: "en-US", Device: iphone69, Paths: shotFiles(t, 1), Timeout: time.Minute})
	if err != nil {
		t.Fatal(err)
	}
	if ch.Action != metadata.ActionUpdate || ch.Old != "1 screenshot" || ch.New != "2 screenshots" || set.Device != "iphone-6.9" || len(set.Screenshots) != 1 {
		t.Errorf("change=%+v set=%+v", ch, set)
	}
	for _, m := range mutations(srv.Server) {
		if strings.HasPrefix(m, "DELETE") {
			t.Errorf("append deleted: %s", m)
		}
	}
}

func TestUploadScreenshotsUnknownLocale(t *testing.T) {
	srv := newShotServer(t, "appScreenshotSets_two.json", nil, instantly)
	svc, _ := uploadService(t, srv.Server)
	_, _, err := svc.UploadScreenshots(t.Context(), Version{ID: "v1", Version: "1.3.0"}, ScreenshotUpload{Locale: "ja", Device: iphone69, Paths: shotFiles(t, 1)})
	var ce *clierr.Error
	if err == nil || !asClierr(err, &ce) || ce.Hint != "store apple listing locale add ja" {
		t.Fatalf("err = %v", err)
	}
}

func asClierr(err error, target **clierr.Error) bool {
	ce, ok := err.(*clierr.Error) //nolint:errorlint // direct type is what the test wants
	*target = ce
	return ok
}

func TestListScreenshots(t *testing.T) {
	srv := newShotServer(t, "appScreenshotSets_two.json", []remoteShot{{"a", "01.png", "s1"}, {"b", "02.png", "s2"}}, instantly)
	svc, _ := uploadService(t, srv.Server)
	sets, err := svc.ListScreenshots(t.Context(), Version{ID: "v1", Version: "1.3.0"}, "", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 4 || sets[0].Locale != "de-DE" || sets[0].Device != "iphone-6.9" || sets[1].Device != "ipad-13" || sets[2].Locale != "en-US" {
		t.Fatalf("sets = %+v", sets)
	}
	if s := sets[0].Screenshots[1]; s.Position != 2 || s.Checksum != "s2" || s.State != "COMPLETE" || s.FileName != "02.png" {
		t.Errorf("screenshot = %+v", s)
	}
	b, _ := json.MarshalIndent(sets[:1], "", "  ")
	golden.Assert(t, "screenshot_set.json", b)

	d := iphone69
	sets, err = svc.ListScreenshots(t.Context(), Version{ID: "v1"}, "en-US", &d)
	if err != nil || len(sets) != 1 {
		t.Fatalf("sets=%+v err=%v", sets, err)
	}
	var filters []string
	for _, r := range srv.Requests() {
		if strings.HasSuffix(r.Path, "/appScreenshotSets") {
			filters = append(filters, url.Values(r.Query).Get("filter[screenshotDisplayType]"))
		}
	}
	if got := filters[len(filters)-1]; got != "APP_IPHONE_67" {
		t.Errorf("filters = %v", filters)
	}
}

func TestDeleteScreenshots(t *testing.T) {
	srv := newShotServer(t, "appScreenshotSets_two.json", []remoteShot{{"a", "01.png", "s1"}}, instantly)
	svc, _ := uploadService(t, srv.Server)
	v := Version{ID: "v1", Version: "1.3.0"}

	_, ch, err := svc.DeleteScreenshots(t.Context(), v, "en-US", iphone69, true)
	if err != nil || ch.Action != metadata.ActionDelete || ch.Old != "1 screenshot" || len(mutations(srv.Server)) != 0 {
		t.Fatalf("dry run: ch=%+v err=%v", ch, err)
	}
	res, _, err := svc.DeleteScreenshots(t.Context(), v, "en-US", iphone69, false)
	if err != nil || res != (ScreenshotsDeleted{Locale: "en-US", Device: "iphone-6.9", Action: metadata.ActionDelete}) {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	if m := mutations(srv.Server); len(m) != 1 || m[0] != "DELETE /v1/appScreenshotSets/set-1" {
		t.Errorf("mutations = %v", m)
	}
}

// A set or locale that does not exist is nothing to do: skip, no error.
func TestDeleteScreenshotsMissingSkips(t *testing.T) {
	v := Version{ID: "v1", Version: "1.3.0"}
	for _, locale := range []string{"en-US", "fr-FR"} {
		for _, dry := range []bool{true, false} {
			srv := newShotServer(t, "appScreenshotSets_empty.json", nil, instantly)
			svc, _ := uploadService(t, srv.Server)
			res, ch, err := svc.DeleteScreenshots(t.Context(), v, locale, iphone69, dry)
			if err != nil || res.Action != metadata.ActionSkip || ch.Action != metadata.ActionSkip || ch.Locale != locale || ch.Field != "iphone-6.9" {
				t.Errorf("%s dry=%v: res=%+v ch=%+v err=%v", locale, dry, res, ch, err)
			}
			if m := mutations(srv.Server); len(m) != 0 {
				t.Errorf("mutations = %v", m)
			}
		}
	}
}

func TestDeviceForScreenshots(t *testing.T) {
	paths := shotFiles(t, 1)
	if d, err := DeviceForScreenshots("ios", "", paths); err != nil || d.Alias != "iphone-6.9" {
		t.Errorf("detect: %v %v", d.Alias, err)
	}
	if d, err := DeviceForScreenshots("ios", "ipad-12.9", paths); err != nil || d.Alias != "ipad-12.9" {
		t.Errorf("alias: %v %v", d.Alias, err)
	}
	if _, err := DeviceForScreenshots("ios", "iphone-9", paths); err == nil || clierr.ExitCode(err) != 2 {
		t.Errorf("unknown alias: %v", err)
	}
	odd := filepath.Join(t.TempDir(), "odd.png")
	pngfake.Write(t, odd, 123, 456, 1)
	if _, err := DeviceForScreenshots("ios", "", []string{odd}); err == nil || clierr.ExitCode(err) != 1 || !strings.Contains(err.Error(), "accepted sizes") {
		t.Errorf("unknown size: %v", err)
	}
}

// A screenshot that an earlier upload left FAILED or unprocessed keeps its
// checksum; the same files must not be treated as in sync.
func TestSyncScreenshotsNotCompleteIsReplaced(t *testing.T) {
	paths := shotFiles(t, 1)
	sm := sums(t, paths)
	for _, state := range []string{"FAILED", "UPLOAD_COMPLETE", "AWAITING_UPLOAD"} {
		t.Run(state, func(t *testing.T) {
			srv := newShotServer(t, "appScreenshotSets_two.json", []remoteShot{{"a", "01.png", sm[0]}}, instantly)
			srv.remoteState = state
			svc, _ := uploadService(t, srv.Server)
			ch, err := svc.SyncScreenshots(t.Context(), "vl-en", "en-US", iphone69, paths, false)
			if err != nil || ch.Action != metadata.ActionReplace {
				t.Fatalf("change=%+v err=%v", ch, err)
			}
			if m := mutations(srv.Server); len(m) == 0 || m[0] != "DELETE /v1/appScreenshots/a" {
				t.Errorf("mutations = %v", m)
			}
		})
	}
}

func TestScreenshotSetsRenderText(t *testing.T) {
	sets := ScreenshotSets{
		{Locale: "en-US", Device: "iphone-6.9", DisplayType: "APP_IPHONE_67", Screenshots: []Screenshot{{ID: "a", FileName: "01.png", Position: 1, Checksum: "c1", State: "COMPLETE"}}},
		{Locale: "en-US", Device: "ipad-13", DisplayType: "APP_IPAD_PRO_3GEN_129", Screenshots: []Screenshot{}},
	}
	var b strings.Builder
	if err := sets.RenderText(&b, output.NewStyles(false)); err != nil {
		t.Fatal(err)
	}
	golden.Assert(t, "screenshot_sets.text", []byte(b.String()))
}
