package cli_test

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"maps"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"
	"testing"

	"github.com/nikeee/store-cli/internal/testutil/golden"
	"github.com/nikeee/store-cli/internal/testutil/playfake"
	"github.com/nikeee/store-cli/internal/testutil/pngfake"
)

const playAppPath = "/androidpublisher/v3/applications/com.example.app"

// playStore is a fake Play listing with state: listings per language and
// images per language and API image type, changed by the requests like an
// edit (PUT replaces a listing, deleteall empties a set, an upload appends an
// image with the SHA-256 of its bytes).
type playStore struct {
	*playfake.Server
	mu       sync.Mutex
	listings map[string]map[string]string // language -> API field -> value
	images   map[string][]string          // "<language>/<imageType>" -> SHA-256 list
}

func newPlayStore(t *testing.T) *playStore {
	s := &playStore{Server: googleEditServer(t), listings: map[string]map[string]string{}, images: map[string][]string{}}
	s.Handle("GET", editListingsPath, s.listListings)
	s.Handle("GET", editListingPath, s.getListing)
	s.Handle("PUT", editListingPath, s.putListing)
	s.Handle("GET", editImagesPath, s.listImages)
	s.Handle("DELETE", editImagesPath, s.deleteImages)
	s.Handle("POST", editImagesUploadPath, s.uploadImage)
	return s
}

func writeJSONBody(w http.ResponseWriter, v any) {
	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(v)
}

func badRequest(w http.ResponseWriter) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusBadRequest)
	_, _ = io.WriteString(w, `{"error":{"code":400,"message":"Invalid request","status":"INVALID_ARGUMENT"}}`)
}

// listing returns the API listing of language; the caller holds mu.
func (s *playStore) listing(language string) map[string]string {
	l := maps.Clone(s.listings[language])
	l["language"] = language
	return l
}

func (s *playStore) listListings(w http.ResponseWriter, _ *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	all := []map[string]string{}
	for _, language := range slices.Sorted(maps.Keys(s.listings)) {
		all = append(all, s.listing(language))
	}
	writeJSONBody(w, map[string]any{"listings": all})
}

func (s *playStore) getListing(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	language := r.PathValue("language")
	if _, ok := s.listings[language]; !ok {
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":{"code":404,"message":"not found","status":"NOT_FOUND"}}`)
		return
	}
	writeJSONBody(w, s.listing(language))
}

func (s *playStore) putListing(w http.ResponseWriter, r *http.Request) {
	var l map[string]string
	if err := json.NewDecoder(r.Body).Decode(&l); err != nil {
		badRequest(w)
		return
	}
	language := r.PathValue("language")
	delete(l, "language")
	s.mu.Lock()
	defer s.mu.Unlock()
	s.listings[language] = l
	writeJSONBody(w, s.listing(language))
}

func imageKey(r *http.Request) string {
	return r.PathValue("language") + "/" + r.PathValue("imageType")
}

func (s *playStore) listImages(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	imgs := []map[string]string{}
	for i, sum := range s.images[imageKey(r)] {
		imgs = append(imgs, map[string]string{"id": fmt.Sprint("img-", i+1), "url": "https://play.example/" + sum, "sha256": sum})
	}
	writeJSONBody(w, map[string]any{"images": imgs})
}

func (s *playStore) deleteImages(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	defer s.mu.Unlock()
	deleted := []map[string]string{}
	for i := range s.images[imageKey(r)] {
		deleted = append(deleted, map[string]string{"id": fmt.Sprint("img-", i+1)})
	}
	delete(s.images, imageKey(r))
	writeJSONBody(w, map[string]any{"deleted": deleted})
}

func (s *playStore) uploadImage(w http.ResponseWriter, r *http.Request) {
	b, _ := io.ReadAll(r.Body)
	s.mu.Lock()
	defer s.mu.Unlock()
	k := imageKey(r)
	s.images[k] = append(s.images[k], sha256Hex(b))
	writeJSONBody(w, map[string]any{"image": map[string]string{"id": fmt.Sprint("img-", len(s.images[k])), "sha256": sha256Hex(b)}})
}

func sha256Hex(b []byte) string {
	s := sha256.Sum256(b)
	return hex.EncodeToString(s[:])
}

// writeListingTree writes files (path relative to root -> content) below
// root; a path ending in .png gets an opaque PNG of the size and shade in the
// content ("WxH/shade"). It returns the SHA-256 of every PNG by path.
func writeListingTree(t *testing.T, root string, files map[string]string) map[string]string {
	t.Helper()
	sums := map[string]string{}
	for p, c := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(p, ".png") {
			if err := os.WriteFile(full, []byte(c), 0o600); err != nil {
				t.Fatal(err)
			}
			continue
		}
		var w, h int
		var shade uint8
		if _, err := fmt.Sscanf(c, "%dx%d/%d", &w, &h, &shade); err != nil {
			t.Fatal(err)
		}
		pngfake.Write(t, full, w, h, shade)
		sums[p] = sha256Hex([]byte(readFile(t, full)))
	}
	return sums
}

// googlePushSetup returns a metadata directory and a store it differs from:
// en-US title, video (cleared) and phone screenshots, a new en-US icon, the
// de-DE full description, and the new locale fr-FR with a title (its empty
// short description has nothing to clear) and a feature graphic. en-US short
// description and the de-DE phone screenshots are equal.
func googlePushSetup(t *testing.T) (dir string, srv *playStore) {
	t.Helper()
	dir = filepath.Join(t.TempDir(), "metadata")
	sums := writeListingTree(t, filepath.Join(dir, "android"), map[string]string{
		"en-US/title.txt":                      "New title\n",
		"en-US/short_description.txt":          "Short\n",
		"en-US/video.txt":                      "\n",
		"en-US/images/phoneScreenshots/01.png": "640x1280/1",
		"en-US/images/phoneScreenshots/02.png": "640x1280/2",
		"en-US/images/icon.png":                "512x512/3",
		"de-DE/full_description.txt":           "Neue Beschreibung\n",
		"de-DE/images/phoneScreenshots/01.png": "640x1280/4",
		"de-DE/images/phoneScreenshots/02.png": "640x1280/5",
		"fr-FR/title.txt":                      "Titre\n",
		"fr-FR/short_description.txt":          "",
		"fr-FR/images/featureGraphic.png":      "1024x500/6",
	})
	srv = newPlayStore(t)
	srv.listings["en-US"] = map[string]string{"title": "Title", "shortDescription": "Short", "fullDescription": "Full", "video": "https://youtu.be/x"}
	srv.listings["de-DE"] = map[string]string{"title": "Titel"}
	srv.images["en-US/phoneScreenshots"] = []string{"0ld"}
	srv.images["de-DE/phoneScreenshots"] = []string{sums["de-DE/images/phoneScreenshots/01.png"], sums["de-DE/images/phoneScreenshots/02.png"]}
	return dir, srv
}

// googlePushPlan is the plan of googlePushSetup.
var googlePushPlan = []string{
	"update listing de-DE fullDescription",
	"update listing en-US title",
	"update listing en-US video",
	"create listing fr-FR title",
	"replace images en-US phone",
	"create images en-US icon",
	"create images fr-FR feature-graphic",
}

func pushArgs(dir string, args ...string) []string {
	return append([]string{"google", "push", dir, "--app", "com.example.app"}, args...)
}

// googleCalls returns method and path (without the app prefix) of every
// request, only the mutating ones if mutating. Upload paths keep "/upload".
func googleCalls(srv *playfake.Server, mutating bool) []string {
	var out []string
	for _, r := range srv.Requests() {
		if mutating && r.Method == http.MethodGet {
			continue
		}
		out = append(out, r.Method+" "+strings.Replace(r.Path, playAppPath, "", 1))
	}
	return out
}

func planLines(t *testing.T, stdout string) []string {
	t.Helper()
	var got []string
	for _, c := range decodePlan(t, stdout).Changes {
		got = append(got, strings.Join([]string{c.Action, c.Scope, c.Locale, c.Field}, " "))
	}
	return got
}

func TestGooglePushPlanAndApply(t *testing.T) {
	dir, srv := googlePushSetup(t)
	code, stdout, stderr := runEnv(t, googleEnv(srv.Server, nil), false, pushArgs(dir, "-o", "json")...)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	golden.Assert(t, "google_push_plan.json", []byte(stdout))
	// one edit: plan, listings, images, commit last
	assertRequests(t, googleCalls(srv.Server, false), []string{
		"POST /edits",
		"GET /edits/edit-1/listings",
		"GET /edits/edit-1/listings/de-DE/phoneScreenshots",
		"GET /edits/edit-1/listings/en-US/phoneScreenshots",
		"GET /edits/edit-1/listings/en-US/icon",
		"GET /edits/edit-1/listings/de-DE",
		"PUT /edits/edit-1/listings/de-DE",
		"GET /edits/edit-1/listings/en-US",
		"PUT /edits/edit-1/listings/en-US",
		"GET /edits/edit-1/listings/fr-FR",
		"PUT /edits/edit-1/listings/fr-FR",
		"GET /edits/edit-1/listings/en-US/phoneScreenshots",
		"DELETE /edits/edit-1/listings/en-US/phoneScreenshots",
		"POST /upload/edits/edit-1/listings/en-US/phoneScreenshots",
		"POST /upload/edits/edit-1/listings/en-US/phoneScreenshots",
		"GET /edits/edit-1/listings/en-US/icon",
		"POST /upload/edits/edit-1/listings/en-US/icon",
		"GET /edits/edit-1/listings/fr-FR/featureGraphic",
		"POST /upload/edits/edit-1/listings/fr-FR/featureGraphic",
		"POST /edits/edit-1:commit",
	})
	if q := srv.Requests()[len(srv.Requests())-1].Query; q.Has("changesNotSentForReview") {
		t.Errorf("commit query = %s", q.Encode())
	}
	wantListings := map[string]map[string]string{
		// untouched fields are kept, an empty file clears its field
		"en-US": {"title": "New title", "shortDescription": "Short", "fullDescription": "Full"},
		"de-DE": {"title": "Titel", "fullDescription": "Neue Beschreibung"},
		"fr-FR": {"title": "Titre"},
	}
	for l, want := range wantListings {
		if got := srv.listings[l]; !maps.Equal(got, want) {
			t.Errorf("listing %s = %v, want %v", l, got, want)
		}
	}
	for k, n := range map[string]int{"en-US/phoneScreenshots": 2, "en-US/icon": 1, "fr-FR/featureGraphic": 1, "de-DE/phoneScreenshots": 2} {
		if got := len(srv.images[k]); got != n {
			t.Errorf("%s holds %d images, want %d", k, got, n)
		}
	}
	if srv.images["en-US/phoneScreenshots"][0] == "0ld" {
		t.Error("en-US phone screenshots not replaced")
	}
}

func TestGooglePushDryRunDeletesEdit(t *testing.T) {
	dir, srv := googlePushSetup(t)
	code, _, stderr := runEnv(t, googleEnv(srv.Server, nil), false, pushArgs(dir, "--dry-run")...)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	// text mode: the plan on stderr, one line per change
	golden.Assert(t, "google_push_plan_stderr.text", []byte(stderr))
	assertRequests(t, googleCalls(srv.Server, true), []string{"POST /edits", "DELETE /edits/edit-1"})

	// json: the plan only on stdout
	dir, srv = googlePushSetup(t)
	code, stdout, stderr := runEnv(t, googleEnv(srv.Server, nil), false, pushArgs(dir, "--dry-run", "-o", "json")...)
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if got := planLines(t, stdout); !slices.Equal(got, googlePushPlan) {
		t.Errorf("plan = %v", got)
	}
	assertRequests(t, googleCalls(srv.Server, true), []string{"POST /edits", "DELETE /edits/edit-1"})
	if srv.listings["en-US"]["title"] != "Title" || srv.images["en-US/phoneScreenshots"][0] != "0ld" {
		t.Errorf("dry run changed the store: %v %v", srv.listings, srv.images)
	}
}

func TestGooglePushDeletesEditOnFailure(t *testing.T) {
	cases := []struct {
		name   string
		fail   func(s *playStore)
		msg    []string
		staged int // changes of googlePushPlan staged before the failure
	}{
		{
			// the listing of the third locale fails
			name: "listing",
			fail: func(s *playStore) {
				s.Handle("PUT", editListingPath, func(w http.ResponseWriter, r *http.Request) {
					if r.PathValue("language") == "fr-FR" {
						badRequest(w)
						return
					}
					s.putListing(w, r)
				})
			},
			msg:    []string{"staged 3 of 7 changes, then the edit was discarded, nothing was published: update listing fr-FR: Invalid request"},
			staged: 3,
		},
		{
			// the second upload fails after the old images were deleted
			name: "image",
			fail: func(s *playStore) {
				n := 0
				s.Handle("POST", editImagesUploadPath, func(w http.ResponseWriter, r *http.Request) {
					if n++; n == 2 {
						badRequest(w)
						return
					}
					s.uploadImage(w, r)
				})
			},
			msg: []string{"staged 4 of 7 changes, then the edit was discarded, nothing was published: en-US phone: upload ",
				"02.png: Invalid request (image set incomplete, run again"},
			staged: 4,
		},
		{
			name: "commit",
			fail: func(s *playStore) {
				s.Handle("POST", googleEditsPath+"/edit-1:commit", func(w http.ResponseWriter, _ *http.Request) { badRequest(w) })
			},
			msg:    []string{"staged 7 of 7 changes, then the edit was discarded, nothing was published: commit edit: Invalid request"},
			staged: 7,
		},
	}
	for _, c := range cases {
		for _, mode := range []string{"text", "json"} {
			t.Run(c.name+"/"+mode, func(t *testing.T) {
				dir, srv := googlePushSetup(t)
				c.fail(srv)
				code, stdout, stderr := runEnv(t, googleEnv(srv.Server, nil), false, pushArgs(dir, "-o", mode)...)
				if code != 1 {
					t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
				}
				for _, m := range c.msg {
					if !strings.Contains(stderr, m) {
						t.Errorf("stderr lacks %q:\n%s", m, stderr)
					}
				}
				calls := googleCalls(srv.Server, true)
				if n := slices.Index(calls, "POST /edits"); n != 0 || slices.Index(calls[1:], "POST /edits") != -1 {
					t.Errorf("not exactly one edit: %v", calls)
				}
				if last := calls[len(calls)-1]; last != "DELETE /edits/edit-1" {
					t.Errorf("last call = %s, want the edit deleted", last)
				}
				if c.name != "commit" && slices.Contains(calls, "POST /edits/edit-1:commit") {
					t.Errorf("committed: %v", calls)
				}
				want := slices.Clone(googlePushPlan)
				for i := c.staged; i < len(want); i++ {
					_, rest, _ := strings.Cut(want[i], " ")
					want[i] = "skip " + rest
				}
				if mode == "json" {
					if got := planLines(t, stdout); !slices.Equal(got, want) {
						t.Errorf("plan = %v, want %v", got, want)
					}
					return
				}
				// after the plan, stderr shows which changes had been staged
				_, result, _ := strings.Cut(stderr, "result (skip = not staged, nothing was published): 7 changes\n")
				for _, line := range want {
					if !strings.Contains(result, "  "+line+"\n") {
						t.Errorf("result lacks %q:\n%s", line, stderr)
					}
				}
				if stdout != "" {
					t.Errorf("stdout = %q", stdout)
				}
			})
		}
	}
}

func TestGooglePushValidationBeforeAnyCall(t *testing.T) {
	cases := []struct {
		name  string
		files map[string]string
		args  []string
		code  int
		msg   string
	}{
		{"too long", map[string]string{"android/de-DE/title.txt": strings.Repeat("ü", 31)}, nil, 1, "de-DE/title.txt: 31 characters, limit 30"},
		{"one phone screenshot", map[string]string{"android/en-US/images/phoneScreenshots/01.png": "640x1280/1"}, nil, 1,
			"en-US: phone needs at least 2 screenshots, got 1"},
		{"wrong icon size", map[string]string{"android/en-US/images/icon.png": "500x500/1"}, nil, 1, "icon must be exactly 512x512"},
		{"no android folder", map[string]string{"en-US/name.txt": "App"}, nil, 1, "android not found"},
		{"bad --only", map[string]string{"android/en-US/title.txt": "App"}, []string{"--only", "texts,screenshots"}, 2, `"screenshots"`},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			dir := filepath.Join(t.TempDir(), "metadata")
			writeListingTree(t, dir, c.files)
			srv := newPlayStore(t)
			code, _, stderr := runEnv(t, googleEnv(srv.Server, nil), false, pushArgs(dir, c.args...)...)
			if code != c.code || !strings.Contains(stderr, c.msg) {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			if n := len(srv.Requests()); n != 0 {
				t.Errorf("%d requests sent", n)
			}
		})
	}
}

func TestGooglePushOnly(t *testing.T) {
	dir, srv := googlePushSetup(t)
	code, stdout, stderr := runEnv(t, googleEnv(srv.Server, nil), false, pushArgs(dir, "--only", "texts", "--changes-not-sent-for-review", "-o", "json")...)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if got := planLines(t, stdout); !slices.Equal(got, googlePushPlan[:4]) {
		t.Errorf("plan = %v", got)
	}
	for _, c := range googleCalls(srv.Server, false) {
		if strings.Count(c, "/") > 4 { // .../listings/<language>/<imageType>
			t.Errorf("texts only requested %s", c)
		}
	}
	if q := srv.Requests()[len(srv.Requests())-1].Query; q.Get("changesNotSentForReview") != "true" {
		t.Errorf("commit query = %s", q.Encode())
	}

	// images of a locale without a listing need its texts
	dir, srv = googlePushSetup(t)
	code, _, stderr = runEnv(t, googleEnv(srv.Server, nil), false, pushArgs(dir, "--only", "images")...)
	if code != 1 || !strings.Contains(stderr, "no listing for fr-FR") || !strings.Contains(stderr, "nothing was published") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	assertRequests(t, googleCalls(srv.Server, true), []string{"POST /edits", "DELETE /edits/edit-1"})

	dir, srv = googlePushSetup(t)
	if err := os.RemoveAll(filepath.Join(dir, "android", "fr-FR", "images")); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr = runEnv(t, googleEnv(srv.Server, nil), false, pushArgs(dir, "--only", "images", "-o", "json")...)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if got := planLines(t, stdout); !slices.Equal(got, googlePushPlan[4:6]) {
		t.Errorf("plan = %v", got)
	}
	if calls := googleCalls(srv.Server, true); slices.ContainsFunc(calls, func(c string) bool { return strings.HasPrefix(c, "PUT") }) {
		t.Errorf("images only changed a listing: %v", calls)
	}
}

// A Play language that is not a locale name never becomes a path.
func TestGooglePullSkipsNonLocaleNames(t *testing.T) {
	srv := newPlayStore(t)
	srv.listings["en-US"] = map[string]string{"title": "Title"}
	srv.listings["../evil"] = map[string]string{"title": "x"}
	dir := filepath.Join(t.TempDir(), "metadata")
	code, stdout, stderr := runEnv(t, googleEnv(srv.Server, nil), false, "google", "pull", dir, "--app", "com.example.app", "-o", "json")
	if code != 0 || strings.Contains(stdout, "evil") || !strings.Contains(stderr, `skipping Play language "../evil"`) {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if _, err := os.Stat(filepath.Join(dir, "evil")); !os.IsNotExist(err) {
		t.Errorf("wrote outside android/: %v", err)
	}
}

func TestGooglePullThenPushIsEmptyPlan(t *testing.T) {
	srv := newPlayStore(t)
	srv.listings["en-US"] = map[string]string{"title": "Title", "shortDescription": "Short", "fullDescription": "Line one\nLine two\n\n"}
	srv.listings["de-DE"] = map[string]string{"title": "Titel"}
	srv.images["en-US/phoneScreenshots"] = []string{"aa", "bb"}
	dir := filepath.Join(t.TempDir(), "metadata")
	// files of an earlier pull with a value Play no longer has, and a file pull does not own
	writeListingTree(t, filepath.Join(dir, "android"), map[string]string{
		"de-DE/short_description.txt": "Old\n",
		"en-US/notes.md":              "mine\n",
	})
	code, stdout, stderr := runEnv(t, googleEnv(srv.Server, nil), false, "google", "pull", dir, "--app", "com.example.app", "-o", "json")
	if code != 0 {
		t.Fatalf("pull: code=%d stderr=%q", code, stderr)
	}
	if want := "{\n  \"locales\": [\n    \"de-DE\",\n    \"en-US\"\n  ],\n  \"files\": 8\n}\n"; stdout != want {
		t.Errorf("stdout = %s", stdout)
	}
	android := filepath.Join(dir, "android")
	for p, want := range map[string]string{
		"en-US/title.txt":            "Title\n",
		"en-US/full_description.txt": "Line one\nLine two\n\n\n",
		// empty in Play: a file that reads as empty, which push reads as "clear"
		"en-US/video.txt":             "\n",
		"de-DE/short_description.txt": "\n",
		"en-US/notes.md":              "mine\n",
	} {
		if got := readFile(t, filepath.Join(android, filepath.FromSlash(p))); got != want {
			t.Errorf("%s = %q, want %q", p, got, want)
		}
	}
	if _, err := os.Stat(filepath.Join(android, "en-US", "images")); !os.IsNotExist(err) {
		t.Errorf("pull wrote images: %v", err)
	}
	pulled := googleCalls(srv.Server, true)
	assertRequests(t, pulled, []string{"POST /edits", "DELETE /edits/edit-1"})

	// nothing to change: the edit is deleted, not committed
	code, stdout, stderr = runEnv(t, googleEnv(srv.Server, nil), false, pushArgs(dir, "-o", "json")...)
	if code != 0 || stdout != "{\n  \"changes\": []\n}\n" || stderr != "" {
		t.Fatalf("push: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	assertRequests(t, googleCalls(srv.Server, true)[len(pulled):], []string{"POST /edits", "DELETE /edits/edit-1"})
}

func TestGoogleInitOffline(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "metadata")
	// no Google env vars at all; it is written as the Play code it-IT
	code, stdout, stderr := runEnv(t, nil, false, "google", "init", dir, "--locale", "en-US", "--locale", "it")
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	golden.Assert(t, "google_init.text", []byte(strings.ReplaceAll(stdout, tmp, "<tmp>")))
	for _, p := range []string{"en-US/title.txt", "it-IT/video.txt"} {
		if got := readFile(t, filepath.Join(dir, "android", filepath.FromSlash(p))); got != "" {
			t.Errorf("%s = %q", p, got)
		}
	}
	for _, p := range []string{"en-US/images", "it-IT/images/phoneScreenshots", "it-IT/images/wearScreenshots"} {
		if fi, err := os.Stat(filepath.Join(dir, "android", filepath.FromSlash(p))); err != nil || !fi.IsDir() {
			t.Errorf("%s: %v", p, err)
		}
	}

	// a second run creates nothing
	code, stdout, _ = runEnv(t, nil, false, "google", "init", dir, "--locale", "en-US", "--locale", "it-IT", "-o", "json")
	var res struct{ Created, Existing []string }
	if err := json.Unmarshal([]byte(stdout), &res); code != 0 || err != nil || len(res.Created) != 0 || len(res.Existing) != 2*(4+6) {
		t.Errorf("code=%d result=%+v err=%v", code, res, err)
	}

	for _, args := range [][]string{{dir}, {dir, "--locale", "en_US"}} {
		code, _, stderr = runEnv(t, nil, false, append([]string{"google", "init"}, args...)...)
		if code != 2 || !strings.Contains(stderr, "--locale") {
			t.Errorf("%v: code=%d stderr=%q", args, code, stderr)
		}
	}
}
