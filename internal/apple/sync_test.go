package apple

import (
	"errors"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/metadata"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
	"github.com/nikeee/store-cli/internal/testutil/pngfake"
)

// writeTree writes files (path relative to root -> content) below root.
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, c := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(full, []byte(c), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

func changeLines(changes []metadata.Change) []string {
	out := make([]string, len(changes))
	for i, c := range changes {
		out[i] = strings.Join([]string{string(c.Action), c.Scope, c.Locale, c.Field}, " ")
	}
	return out
}

func TestReadPushDirReportsAllProblems(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"metadata/de-DE/subtitle.txt":                   strings.Repeat("ü", 31),
		"metadata/en-US/name.txt":                       "\n",
		"metadata/review_information/demo_required.txt": "yes",
	})
	alpha := filepath.Join(root, "screenshots", "en-US", "iphone-6.9", "01.png")
	if err := os.MkdirAll(filepath.Dir(alpha), 0o755); err != nil {
		t.Fatal(err)
	}
	pngfake.WriteAlpha(t, alpha, 1290, 2796)
	alpha2 := filepath.Join(filepath.Dir(alpha), "02.png")
	pngfake.WriteAlpha(t, alpha2, 1290, 2796)
	odd := filepath.Join(root, "screenshots", "de-DE", "01.png")
	if err := os.MkdirAll(filepath.Join(root, "screenshots", "de-DE", "iphone-99"), 0o755); err != nil {
		t.Fatal(err)
	}
	pngfake.Write(t, odd, 100, 100, 1)

	_, err := ReadPushDir(PushOptions{Dir: filepath.Join(root, "metadata"), Platform: "ios"})
	if err == nil || clierr.ExitCode(err) != 1 {
		t.Fatalf("err = %v", err)
	}
	for _, want := range []string{
		"de-DE/subtitle.txt: 31 characters, limit 30",
		"en-US/name.txt: must not be empty",
		"review_information/demo_required.txt: must be true or false",
		alpha + ": has an alpha channel",
		alpha2 + ": has an alpha channel",
		odd + ": no ios device accepts 100x100",
		`unknown device folder "iphone-99"`,
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q: %v", want, err)
		}
	}
}

func TestReadPushDirOnly(t *testing.T) {
	dir := filepath.Join("testdata", "push", "too-long", "metadata")
	_, err := ReadPushDir(PushOptions{Dir: dir, Platform: "ios", Only: []string{"texts", "images"}})
	if err == nil || clierr.ExitCode(err) != 2 || !strings.Contains(err.Error(), `"images"`) {
		t.Fatalf("err = %v", err)
	}
	// texts that are not pushed are not validated
	if _, err := ReadPushDir(PushOptions{Dir: dir, Platform: "ios", Only: []string{"review", "screenshots"}}); err != nil {
		t.Fatalf("err = %v", err)
	}
}

// newLocaleServer serves an app whose version has only en-US until a version
// localization is created, then also fr-FR (vl-new), with empty screenshot sets.
func newLocaleServer(t *testing.T) *shotServer {
	t.Helper()
	ss := newShotServer(t, "appScreenshotSets_empty.json", nil, instantly)
	srv := ss.Server
	var created atomic.Bool
	srv.Handle("GET", "/v1/apps/{id}/appStoreVersions", ascfake.JSON(http.StatusOK, "app_appStoreVersions.json"))
	srv.Handle("GET", "/v1/apps/{id}/appInfos", ascfake.JSON(http.StatusOK, "app_appInfos.json"))
	srv.Handle("GET", "/v1/appInfos/{id}/appInfoLocalizations", ascfake.JSON(http.StatusOK, "appInfo_localizations_en_only.json"))
	srv.Handle("POST", "/v1/appInfoLocalizations", ascfake.JSON(http.StatusCreated, "appInfoLocalization_created.json"))
	srv.Handle("GET", "/v1/appStoreVersions/{id}/appStoreVersionLocalizations", func(w http.ResponseWriter, r *http.Request) {
		if !created.Load() {
			ascfake.JSON(http.StatusOK, "version_localizations_en_only.json")(w, r)
			return
		}
		writeJSON(w, http.StatusOK, map[string]any{"data": []any{
			map[string]any{"type": "appStoreVersionLocalizations", "id": "vl-en", "attributes": map[string]any{"locale": "en-US"}},
			map[string]any{"type": "appStoreVersionLocalizations", "id": "vl-new", "attributes": map[string]any{"locale": "fr-FR"}},
		}, "links": map[string]any{"self": "x"}})
	})
	srv.Handle("POST", "/v1/appStoreVersionLocalizations", func(w http.ResponseWriter, r *http.Request) {
		created.Store(true)
		ascfake.JSON(http.StatusCreated, "versionLocalization_created.json")(w, r)
	})
	return ss
}

func TestPushScreenshotsOfNewLocale(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"metadata/fr-FR/name.txt": "Mon App", "metadata/fr-FR/description.txt": "Une app."})
	if err := os.MkdirAll(filepath.Join(root, "screenshots", "fr-FR"), 0o755); err != nil {
		t.Fatal(err)
	}
	pngfake.Write(t, filepath.Join(root, "screenshots", "fr-FR", "01.png"), 1290, 2796, 3)
	o := PushOptions{Dir: filepath.Join(root, "metadata"), Platform: "ios"}

	// screenshots alone cannot create the localization
	srv := newLocaleServer(t)
	_, err := newService(t, srv.Server).Push(t.Context(), listingApp, PushOptions{Dir: o.Dir, Platform: "ios", Only: []string{"screenshots"}})
	var ce *clierr.Error
	if !errors.As(err, &ce) || !strings.Contains(ce.Message, "no localization for fr-FR") || ce.Hint != "store apple listing locale add fr-FR" {
		t.Fatalf("err = %v", err)
	}
	if m := mutations(srv.Server); len(m) != 0 {
		t.Errorf("mutations = %v", m)
	}

	srv = newLocaleServer(t)
	plan, err := newService(t, srv.Server).Push(t.Context(), listingApp, o)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"create app-info fr-FR name", "create version fr-FR description", "create screenshots fr-FR iphone-6.9"}
	if got := changeLines(plan.Changes); !slices.Equal(got, want) {
		t.Errorf("plan = %v, want %v", got, want)
	}
	wantReq := []string{
		"POST /v1/appInfoLocalizations",
		"POST /v1/appStoreVersionLocalizations",
		"POST /v1/appScreenshotSets",
		"POST /v1/appScreenshots",
		"PATCH /v1/appScreenshots/new-1",
	}
	if got := mutations(srv.Server); !slices.Equal(got, wantReq) {
		t.Errorf("mutations = %v, want %v", got, wantReq)
	}
	for _, r := range requestsOf(srv.Server, "POST") {
		if r.Path == "/v1/appScreenshotSets" && !strings.Contains(string(r.Body), `"id":"vl-new"`) {
			t.Errorf("set created on %s", r.Body)
		}
	}
}

func TestPushReviewWithoutDetail(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"metadata/review_information/first_name.txt":    "Ada",
		"metadata/review_information/phone_number.txt":  "",
		"metadata/review_information/demo_required.txt": "",
		"metadata/review_information/demo_password.txt": "pw",
	})
	srv := reviewServer(t, "")
	srv.Handle("GET", "/v1/apps/{id}/appStoreVersions", ascfake.JSON(http.StatusOK, "app_appStoreVersions.json"))
	o := PushOptions{Dir: filepath.Join(root, "metadata"), Platform: "ios", Version: "1.3.0", Only: []string{"review"}}

	// nothing to clear on a detail that does not exist yet; the password is masked
	plan, err := newService(t, srv).Push(t.Context(), listingApp, o)
	if err != nil {
		t.Fatal(err)
	}
	if got := changeLines(plan.Changes); !slices.Equal(got, []string{"create review  contactFirstName", "create review  demoAccountPassword"}) {
		t.Errorf("plan = %v", got)
	}
	if plan.Changes[1].New != "***" {
		t.Errorf("password = %q", plan.Changes[1].New)
	}
	posts := requestsOf(srv, "POST")
	if len(posts) != 1 || !strings.Contains(string(posts[0].Body), `"attributes":{"contactFirstName":"Ada","demoAccountPassword":"pw"}`) {
		t.Errorf("posts = %v", posts)
	}
}

func TestPushDryRunOnlyReads(t *testing.T) {
	srv := newLocaleServer(t)
	root := t.TempDir()
	writeTree(t, root, map[string]string{"metadata/fr-FR/name.txt": "Mon App"})
	plan, err := newService(t, srv.Server).Push(t.Context(), listingApp, PushOptions{Dir: filepath.Join(root, "metadata"), Platform: "ios", DryRun: true})
	if err != nil || len(plan.Changes) != 1 {
		t.Fatalf("plan=%v err=%v", plan, err)
	}
	if m := mutations(srv.Server); len(m) != 0 {
		t.Errorf("mutations = %v", m)
	}
}

func TestInitScaffolds(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "metadata")
	res, err := Init(dir, "", []string{"en-US"})
	if err != nil {
		t.Fatal(err)
	}
	if n := len(metadata.AppleFields) + len(metadata.AppleReviewFields) + 1; len(res.Created) != n || len(res.Existing) != 0 {
		t.Fatalf("result = %+v", res)
	}
	for _, p := range []string{
		filepath.Join(dir, "en-US", "keywords.txt"),
		filepath.Join(dir, metadata.ReviewDir, "notes.txt"),
		filepath.Join(filepath.Dir(dir), "screenshots", "en-US"),
	} {
		if !slices.Contains(res.Created, p) {
			t.Errorf("%s not created: %v", p, res.Created)
		}
		if _, err := os.Stat(p); err != nil {
			t.Error(err)
		}
	}
}

func TestPushSkipsWhatsNewBeforeFirstRelease(t *testing.T) {
	root := t.TempDir()
	writeTree(t, root, map[string]string{"metadata/en-US/release_notes.txt": "Fixes", "metadata/en-US/description.txt": "Changed."})
	o := PushOptions{Dir: filepath.Join(root, "metadata"), Platform: "ios", Version: "1.3.0", DryRun: true}
	cases := []struct {
		versions string
		want     []string
		warned   bool
	}{
		{"versions_two_editable.json", []string{"update version en-US description"}, true},
		{"app_appStoreVersions.json", []string{"update version en-US description", "update version en-US whatsNew"}, false},
	}
	for _, c := range cases {
		t.Run(c.versions, func(t *testing.T) {
			srv := listingServer(t, "app_appInfos.json", "appInfo_localizations_full.json", "version_localizations.json")
			srv.Handle("GET", "/v1/apps/{id}/appStoreVersions", ascfake.JSON(http.StatusOK, c.versions))
			svc, stderr := uploadService(t, srv)
			plan, err := svc.Push(t.Context(), listingApp, o)
			if err != nil {
				t.Fatal(err)
			}
			if got := changeLines(plan.Changes); !slices.Equal(got, c.want) {
				t.Errorf("plan = %v, want %v", got, c.want)
			}
			const warning = "warning: skipping What's New: App Store Connect accepts it only after the first release\n"
			if got := stderr.String(); (got == warning) != c.warned {
				t.Errorf("stderr = %q", got)
			}
		})
	}
}

func TestPushIgnoresTrailingWhitespace(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/apps/{id}/appStoreVersions", ascfake.JSON(http.StatusOK, "app_appStoreVersions.json"))
	srv.Handle("GET", "/v1/apps/{id}/appInfos", ascfake.JSON(http.StatusOK, "app_appInfos.json"))
	srv.Handle("GET", "/v1/appInfos/{id}/appInfoLocalizations", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"data": []any{map[string]any{"type": "appInfoLocalizations", "id": "il-en",
			"attributes": map[string]any{"locale": "en-US", "name": "Example App", "subtitle": "Do things \t"}}}, "links": map[string]any{"self": "x"}})
	})
	srv.Handle("GET", "/v1/appStoreVersions/{id}/appStoreVersionLocalizations", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"data": []any{map[string]any{"type": "appStoreVersionLocalizations", "id": "vl-en",
			"attributes": map[string]any{"locale": "en-US", "description": "An app.\n\n"}}}, "links": map[string]any{"self": "x"}})
	})
	srv.Handle("GET", "/v1/appStoreVersions/{id}/appStoreReviewDetail", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"data": map[string]any{"type": "appStoreReviewDetails", "id": "rd-1",
			"attributes": map[string]any{"notes": "Use the demo.\r\n"}}, "links": map[string]any{"self": "x"}})
	})
	// files as pull writes them: value plus a newline, trimmed when read
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"metadata/en-US/subtitle.txt":           "Do things \t\n",
		"metadata/en-US/description.txt":        "An app.\n\n\n",
		"metadata/review_information/notes.txt": "Use the demo.\r\n\n",
	})
	plan, err := newService(t, srv).Push(t.Context(), listingApp, PushOptions{Dir: filepath.Join(root, "metadata"), Platform: "ios"})
	if err != nil || len(plan.Changes) != 0 {
		t.Fatalf("plan=%v err=%v", plan, err)
	}
}
