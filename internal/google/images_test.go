package google

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/media"
	"github.com/nikeee/store-cli/internal/metadata"
	"github.com/nikeee/store-cli/internal/testutil/playfake"
	"github.com/nikeee/store-cli/internal/testutil/pngfake"
)

const (
	imagesPath       = listingsPath + "/{language}/{imageType}"
	imagesUploadPath = "/upload/androidpublisher/v3/applications/{packageName}/edits/edit-1/listings/{language}/{imageType}"
)

// imageServer keeps the images of one (language, type) like Play: list,
// deleteall and upload (appending, with the SHA-256 of the uploaded bytes).
// failUpload makes the n-th upload (1 based) fail with a 500.
type imageServer struct {
	*playfake.Server
	mu         sync.Mutex
	images     []map[string]string
	uploads    [][]byte
	failUpload int
}

func newImageServer(t *testing.T, remote ...string) *imageServer {
	s := &imageServer{Server: editServer(t)}
	for i, sum := range remote {
		s.images = append(s.images, map[string]string{"id": "img-" + string(rune('a'+i)), "url": "https://img/" + sum, "sha256": sum})
	}
	s.Handle("GET", imagesPath, func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"images": s.images})
	})
	s.Handle("DELETE", imagesPath, func(w http.ResponseWriter, _ *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		deleted := s.images
		s.images = nil
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"deleted": deleted})
	})
	s.Handle("POST", imagesUploadPath, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		defer s.mu.Unlock()
		s.uploads = append(s.uploads, b)
		if s.failUpload == len(s.uploads) {
			w.WriteHeader(http.StatusInternalServerError)
			_, _ = io.WriteString(w, `{"error":{"code":500,"message":"boom","status":"INTERNAL"}}`)
			return
		}
		sum := sha256.Sum256(b)
		img := map[string]string{"id": "up-" + string(rune('0'+len(s.uploads))), "url": "https://img/up", "sha256": hex.EncodeToString(sum[:])}
		s.images = append(s.images, img)
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"image": img})
	})
	return s
}

func sumOf(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// shots writes opaque 640x1280 PNGs with the given shades.
func shots(t *testing.T, w, h int, shades ...uint8) (paths []string, sums []string) {
	t.Helper()
	dir := t.TempDir()
	for i, sh := range shades {
		p := filepath.Join(dir, string(rune('a'+i))+".png")
		pngfake.Write(t, p, w, h, sh)
		b, err := os.ReadFile(p)
		if err != nil {
			t.Fatal(err)
		}
		paths = append(paths, p)
		sums = append(sums, sumOf(b))
	}
	return paths, sums
}

func phoneType() media.ImageType {
	ty, _ := media.ImageTypeByAlias("phone")
	return ty
}

func TestSyncImagesUnchanged(t *testing.T) {
	paths, sums := shots(t, 640, 1280, 1, 2)
	srv := newImageServer(t, sums...)
	var change metadata.Change
	s := newService(t, srv.Server, io.Discard)
	err := s.withEdit(t.Context(), CommitOptions{}, func(editID string) error {
		var err error
		change, err = s.SyncImages(t.Context(), editID, "en-US", phoneType(), paths, false)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if change.Action != metadata.ActionSkip || change.Scope != "images" || change.Old != "2 images" || change.New != "2 images" {
		t.Errorf("change = %+v", change)
	}
	for _, c := range calls(srv.Server) {
		if strings.Contains(c, "/listings/") && !strings.HasPrefix(c, "GET") {
			t.Errorf("unexpected call %s", c)
		}
	}
}

func TestSyncImagesReplacesInOrder(t *testing.T) {
	paths, sums := shots(t, 640, 1280, 1, 2, 3)
	// Play holds the same files in another order: different, so replaced.
	srv := newImageServer(t, sums[2], sums[0])
	var change metadata.Change
	s := newService(t, srv.Server, io.Discard)
	err := s.withEdit(t.Context(), CommitOptions{}, func(editID string) error {
		var err error
		change, err = s.SyncImages(t.Context(), editID, "en-US", phoneType(), paths, false)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if change.Action != metadata.ActionReplace || change.Old != "2 images" || change.New != "3 images" || change.Locale != "en-US" || change.Field != "phone" {
		t.Errorf("change = %+v", change)
	}
	const uploadCall = "/upload/androidpublisher/v3/applications/com.example.app/edits/edit-1/listings/en-US/phoneScreenshots"
	want := []string{
		"POST /edits",
		"GET /edits/edit-1/listings/en-US/phoneScreenshots",
		"DELETE /edits/edit-1/listings/en-US/phoneScreenshots",
		"POST " + uploadCall,
		"POST " + uploadCall,
		"POST " + uploadCall,
		"POST /edits/edit-1:commit",
	}
	got := calls(srv.Server)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Errorf("calls = %v", got)
	}
	if len(srv.uploads) != 3 {
		t.Fatalf("uploads = %d", len(srv.uploads))
	}
	// The fake hands the handler only the media; the raw multipart body
	// carries the content type of the part.
	for _, r := range srv.Requests() {
		if strings.HasPrefix(r.Path, "/upload/") && !strings.Contains(string(r.Body), "Content-Type: image/png") {
			t.Errorf("upload without image/png content type: %.200q", r.Body)
		}
	}
	for i, b := range srv.uploads {
		if !strings.HasPrefix(string(b), "\x89PNG") {
			t.Errorf("upload %d is not the PNG file", i)
		}
	}
	for i, img := range srv.images {
		if img["sha256"] != sums[i] {
			t.Errorf("image %d has sha256 %s, want %s", i, img["sha256"], sums[i])
		}
	}
}

func TestSyncImagesAppend(t *testing.T) {
	paths, sums := shots(t, 640, 1280, 5)
	srv := newImageServer(t, "aa", "bb")
	s := newService(t, srv.Server, io.Discard)
	_, _, err := s.UploadImages(t.Context(), "en-US", phoneType(), paths, false, CommitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(srv.images) != 3 || srv.images[2]["sha256"] != sums[0] {
		t.Errorf("images = %v", srv.images)
	}
	for _, c := range calls(srv.Server) {
		if strings.HasPrefix(c, "DELETE /edits/edit-1/listings") {
			t.Errorf("append deleted images: %s", c)
		}
	}
}

func TestSyncImagesAppendExceedsMax(t *testing.T) {
	paths, _ := shots(t, 640, 1280, 5, 6)
	srv := newImageServer(t, "1", "2", "3", "4", "5", "6", "7")
	s := newService(t, srv.Server, io.Discard)
	_, _, err := s.UploadImages(t.Context(), "en-US", phoneType(), paths, false, CommitOptions{})
	if err == nil || !strings.Contains(err.Error(), "maximum of 8") || clierr.ExitCode(err) != 1 {
		t.Fatalf("err = %v", err)
	}
	if len(srv.uploads) != 0 {
		t.Error("uploaded despite the limit")
	}
}

func TestImagesAspectRejectedLocally(t *testing.T) {
	good, _ := shots(t, 640, 1280, 1)
	bad, _ := shots(t, 320, 1000, 2) // aspect 3.1:1
	srv := newImageServer(t)
	s := newService(t, srv.Server, io.Discard)
	_, _, err := s.UploadImages(t.Context(), "en-US", phoneType(), append(good, bad...), true, CommitOptions{})
	if err == nil || !strings.Contains(err.Error(), "2x the short side") || clierr.ExitCode(err) != 1 {
		t.Fatalf("err = %v", err)
	}
	if n := len(srv.Requests()); n != 0 {
		t.Errorf("%d requests: %v", n, calls(srv.Server))
	}
}

func TestImageCountsRejectedLocally(t *testing.T) {
	nine := make([]uint8, 9)
	for i := range nine {
		nine[i] = uint8(i + 1)
	}
	many, _ := shots(t, 640, 1280, nine...)
	one, _ := shots(t, 640, 1280, 1)
	icon, _ := media.ImageTypeByAlias("icon")
	two, _ := shots(t, 512, 512, 1, 2)
	tests := []struct {
		name  string
		typ   media.ImageType
		paths []string
		want  string
	}{
		{"too many", phoneType(), many, "at most 8"},
		{"phone needs two", phoneType(), one, "at least 2"},
		{"single type takes one", icon, two, "exactly one"},
		{"none", phoneType(), nil, "no image files"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := newImageServer(t)
			_, _, err := newService(t, srv.Server, io.Discard).UploadImages(t.Context(), "en-US", tt.typ, tt.paths, true, CommitOptions{})
			if err == nil || !strings.Contains(err.Error(), tt.want) || clierr.ExitCode(err) != 1 {
				t.Fatalf("err = %v", err)
			}
			if n := len(srv.Requests()); n != 0 {
				t.Errorf("%d requests", n)
			}
		})
	}
}

func TestFeatureGraphicSize(t *testing.T) {
	ty, _ := media.ImageTypeByAlias("feature-graphic")
	bad, _ := shots(t, 1024, 501, 1)
	srv := newImageServer(t)
	s := newService(t, srv.Server, io.Discard)
	_, _, err := s.UploadImages(t.Context(), "en-US", ty, bad, false, CommitOptions{})
	if err == nil || !strings.Contains(err.Error(), "exactly 1024x500") || len(srv.Requests()) != 0 {
		t.Fatalf("err = %v, requests = %d", err, len(srv.Requests()))
	}

	// A single-image type replaces its one image without --replace.
	good, sums := shots(t, 1024, 500, 7)
	srv = newImageServer(t, "old")
	set, _, err := newService(t, srv.Server, io.Discard).UploadImages(t.Context(), "en-US", ty, good, false, CommitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(set.Images) != 1 || set.Images[0].SHA256 != sums[0] || set.Type != "feature-graphic" {
		t.Errorf("set = %+v", set)
	}
}

func TestSyncImagesDryRunOnlyReads(t *testing.T) {
	paths, _ := shots(t, 640, 1280, 1, 2)
	srv := newImageServer(t, "old")
	s := newService(t, srv.Server, io.Discard)
	_, change, err := s.UploadImages(t.Context(), "en-US", phoneType(), paths, true, CommitOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	if change.Action != metadata.ActionReplace || change.New != "2 images" {
		t.Errorf("change = %+v", change)
	}
	for _, c := range calls(srv.Server) {
		if strings.HasPrefix(c, "DELETE /edits/edit-1/listings") || strings.Contains(c, "commit") || (strings.HasPrefix(c, "POST") && strings.Contains(c, "listings")) {
			t.Errorf("dry run sent %s", c)
		}
	}
}

func TestSyncImagesPartialFailure(t *testing.T) {
	paths, _ := shots(t, 640, 1280, 1, 2)
	srv := newImageServer(t, "old")
	srv.failUpload = 2
	s := newService(t, srv.Server, io.Discard)
	_, _, err := s.UploadImages(t.Context(), "en-US", phoneType(), paths, true, CommitOptions{})
	if err == nil {
		t.Fatal("want error")
	}
	for _, want := range []string{"image set incomplete", "run again", "nothing was published", paths[1]} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("err %q lacks %q", err, want)
		}
	}
	got := calls(srv.Server)
	if last := got[len(got)-1]; last != "DELETE /edits/edit-1" {
		t.Errorf("last call = %s, want the edit deleted", last)
	}
}

func TestDeleteImages(t *testing.T) {
	srv := newImageServer(t, "a", "b")
	s := newService(t, srv.Server, io.Discard)
	res, _, err := s.DeleteImages(t.Context(), "en-US", phoneType(), CommitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if res.Deleted != 2 || res.Locale != "en-US" || res.Type != "phone" {
		t.Errorf("res = %+v", res)
	}
	if len(srv.images) != 0 {
		t.Error("images left")
	}

	srv = newImageServer(t)
	_, _, err = newService(t, srv.Server, io.Discard).DeleteImages(t.Context(), "en-US", phoneType(), CommitOptions{})
	if err == nil || !strings.Contains(err.Error(), "no phone images") {
		t.Errorf("err = %v", err)
	}
}

func TestListImages(t *testing.T) {
	srv := newImageServer(t, "aa")
	s := newService(t, srv.Server, io.Discard)
	ty := phoneType()
	sets, err := s.ListImages(t.Context(), "en-US", &ty)
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != 1 || sets[0].Type != "phone" || sets[0].Images[0] != (Image{ID: "img-a", URL: "https://img/aa", SHA256: "aa"}) {
		t.Errorf("sets = %+v", sets)
	}
	// Without a type only the types that hold images are listed.
	sets, err = s.ListImages(t.Context(), "en-US", nil)
	if err != nil {
		t.Fatal(err)
	}
	if len(sets) != len(media.ImageTypes) { // the fake serves the same images for every type
		t.Errorf("got %d sets", len(sets))
	}
}
