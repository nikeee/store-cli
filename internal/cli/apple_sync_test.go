package cli_test

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/testutil/ascfake"
	"github.com/nikeee/store-cli/internal/testutil/golden"
	"github.com/nikeee/store-cli/internal/testutil/pngfake"
)

const (
	pushVersionID = "7d5f9a2e-3c41-4b8e-9f20-1a6b8c4d0130"
	pushInfoID    = "b7d3f1a0-edit-4c2d-9b4e-000000000002"
)

// pushCLIServer serves the listing of listingCLIServer, the review detail of
// reviewCLIServer and the screenshots of screenshotCLIServer.
func pushCLIServer(t *testing.T) *ascfake.Server {
	t.Helper()
	srv := screenshotCLIServer(t)
	srv.Handle("GET", "/v1/apps/{id}/appInfos", ascfake.JSON(http.StatusOK, "app_appInfos.json"))
	srv.Handle("GET", "/v1/appInfos/{id}/appInfoLocalizations", ascfake.JSON(http.StatusOK, "appInfo_localizations_full.json"))
	srv.Handle("PATCH", "/v1/appInfoLocalizations/{id}", ascfake.JSON(http.StatusOK, "appInfoLocalization_created.json"))
	srv.Handle("POST", "/v1/appInfoLocalizations", ascfake.JSON(http.StatusCreated, "appInfoLocalization_created.json"))
	srv.Handle("PATCH", "/v1/appStoreVersionLocalizations/{id}", ascfake.JSON(http.StatusOK, "versionLocalization_created.json"))
	srv.Handle("POST", "/v1/appStoreVersionLocalizations", ascfake.JSON(http.StatusCreated, "versionLocalization_created.json"))
	detail := func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(reviewCLIDetail))
	}
	srv.Handle("GET", "/v1/appStoreVersions/{id}/appStoreReviewDetail", detail)
	srv.Handle("PATCH", "/v1/appStoreReviewDetails/{id}", detail)
	return srv
}

// pushDir is a sample metadata directory of internal/apple/testdata/push.
func pushDir(name string) string {
	return filepath.Join("..", "apple", "testdata", "push", name, "metadata")
}

// pushShots writes one iphone-6.9 screenshot for en-US and returns the
// screenshots directory.
func pushShots(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "en-US"), 0o755); err != nil {
		t.Fatal(err)
	}
	pngfake.Write(t, filepath.Join(dir, "en-US", "01.png"), 1290, 2796, 7)
	return dir
}

// apiRequests returns method and path of every API request (no uploads),
// only the mutating ones if mutating.
func apiRequests(srv *ascfake.Server, mutating bool) []string {
	var out []string
	for _, r := range srv.Requests() {
		if strings.HasPrefix(r.Path, "/upload/") || (mutating && r.Method == http.MethodGet) {
			continue
		}
		out = append(out, r.Method+" "+r.Path)
	}
	return out
}

func assertRequests(t *testing.T, got, want []string) {
	t.Helper()
	if !slices.Equal(got, want) {
		t.Errorf("requests:\n%s\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

type pushPlan struct {
	Changes []struct{ Action, Scope, Locale, Field, Old, New string }
}

func decodePlan(t *testing.T, stdout string) pushPlan {
	t.Helper()
	var p pushPlan
	if err := json.Unmarshal([]byte(stdout), &p); err != nil {
		t.Fatalf("decode %q: %v", stdout, err)
	}
	return p
}

func TestApplePushPlanAndApply(t *testing.T) {
	srv := pushCLIServer(t)
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "push", pushDir("changes"), "--screenshots", pushShots(t), "-o", "json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	golden.Assert(t, "apple_push_plan.json", []byte(stdout))
	if strings.Contains(stdout+stderr, "newpass") || strings.Contains(stdout+stderr, "s3cret") {
		t.Errorf("password in output: stdout=%s stderr=%s", stdout, stderr)
	}
	ver := "/v1/appStoreVersions/" + pushVersionID
	assertRequests(t, apiRequests(srv, false), []string{
		// plan: version, texts, app info editable, review, screenshots
		"GET /v1/apps",
		"GET /v1/apps/1234567890/appStoreVersions",
		"GET /v1/apps/1234567890/appInfos",
		"GET /v1/appInfos/" + pushInfoID + "/appInfoLocalizations",
		"GET " + ver + "/appStoreVersionLocalizations",
		"GET /v1/apps/1234567890/appInfos",
		"GET " + ver + "/appStoreReviewDetail",
		"GET " + ver + "/appStoreVersionLocalizations",
		"GET /v1/appStoreVersionLocalizations/vl-en/appScreenshotSets",
		"GET /v1/appScreenshotSets/set-1/appScreenshots",
		// apply: app info texts, version texts, review, screenshots
		"GET /v1/apps/1234567890/appInfos",
		"GET /v1/appInfos/" + pushInfoID + "/appInfoLocalizations",
		"PATCH /v1/appInfoLocalizations/il-en",
		"POST /v1/appInfoLocalizations",
		"GET " + ver + "/appStoreVersionLocalizations",
		"PATCH /v1/appStoreVersionLocalizations/vl-de",
		"PATCH /v1/appStoreVersionLocalizations/vl-en",
		"POST /v1/appStoreVersionLocalizations",
		"GET " + ver + "/appStoreReviewDetail",
		"PATCH /v1/appStoreReviewDetails/rd-1",
		"GET " + ver + "/appStoreVersionLocalizations",
		"GET /v1/appStoreVersionLocalizations/vl-en/appScreenshotSets",
		"GET /v1/appScreenshotSets/set-1/appScreenshots",
		"DELETE /v1/appScreenshots/shot-a",
		"DELETE /v1/appScreenshots/shot-b",
		"POST /v1/appScreenshots",
		"PATCH /v1/appScreenshots/new-1",
		"GET /v1/appScreenshots/new-1",
	})
	bodies := map[string]string{}
	for _, r := range srv.Requests() {
		if r.Method != http.MethodGet {
			bodies[r.Method+" "+r.Path] = string(r.Body)
		}
	}
	for req, want := range map[string]string{
		// only the changed attributes; an empty file clears its field
		"PATCH /v1/appInfoLocalizations/il-en":         `{"data":{"type":"appInfoLocalizations","id":"il-en","attributes":{"subtitle":"Do more things"}}}`,
		"PATCH /v1/appStoreVersionLocalizations/vl-en": `{"data":{"type":"appStoreVersionLocalizations","id":"vl-en","attributes":{"keywords":"example,app,new","marketingUrl":null}}}`,
		"PATCH /v1/appStoreReviewDetails/rd-1":         `{"data":{"type":"appStoreReviewDetails","id":"rd-1","attributes":{"contactFirstName":"Grace","demoAccountPassword":"newpass"}}}`,
		"POST /v1/appInfoLocalizations":                `"name":"Mon App"`,
		"POST /v1/appStoreVersionLocalizations":        `"description":"Une app d'exemple."`,
		"PATCH /v1/appStoreVersionLocalizations/vl-de": `"description":"Eine neue Beispiel-App."`,
		"POST /v1/appScreenshots":                      `"fileName":"01.png"`,
	} {
		if got, ok := bodies[req]; !ok || !strings.Contains(got, want) {
			t.Errorf("%s body = %s, want %s", req, got, want)
		}
	}
}

func TestApplePushDryRunNoMutations(t *testing.T) {
	srv := pushCLIServer(t)
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "push", pushDir("changes"), "--screenshots", pushShots(t), "--dry-run")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	// text mode: the plan on stderr, one line per change
	golden.Assert(t, "apple_push_plan_stderr.text", []byte(stderr))
	if strings.Contains(stdout+stderr, "newpass") || !strings.Contains(stdout, "***") {
		t.Errorf("password not masked: stdout=%s", stdout)
	}
	if got := apiRequests(srv, true); len(got) != 0 {
		t.Errorf("dry run sent %v", got)
	}

	// json: the plan only on stdout
	srv = pushCLIServer(t)
	code, stdout, stderr = runEnv(t, buildEnv(srv), false, "apple", "push", pushDir("changes"), "--screenshots", pushShots(t), "--dry-run", "-o", "json")
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if p := decodePlan(t, stdout); len(p.Changes) != 9 {
		t.Errorf("plan = %s", stdout)
	}
	if got := apiRequests(srv, true); len(got) != 0 {
		t.Errorf("dry run sent %v", got)
	}
}

func TestApplePushValidationBeforeAnyCall(t *testing.T) {
	emptyName := filepath.Join(t.TempDir(), "metadata")
	if err := os.MkdirAll(filepath.Join(emptyName, "en-US"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(emptyName, "en-US", "name.txt"), []byte("\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	alpha := t.TempDir()
	if err := os.MkdirAll(filepath.Join(alpha, "en-US", "iphone-6.9"), 0o755); err != nil {
		t.Fatal(err)
	}
	pngfake.WriteAlpha(t, filepath.Join(alpha, "en-US", "iphone-6.9", "01.png"), 1290, 2796)
	cases := []struct {
		name string
		args []string
		msg  string
	}{
		{"too long", []string{pushDir("too-long")}, "de-DE/subtitle.txt: 31 characters, limit 30"},
		{"empty name", []string{emptyName}, "en-US/name.txt: must not be empty"},
		{"alpha screenshot", []string{pushDir("changes"), "--screenshots", alpha}, "alpha channel"},
		{"missing dir", []string{filepath.Join(t.TempDir(), "nope")}, "not found"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := pushCLIServer(t)
			code, _, stderr := runEnv(t, buildEnv(srv), false, append([]string{"apple", "push"}, c.args...)...)
			if code != 1 || !strings.Contains(stderr, c.msg) {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			if n := len(srv.Requests()); n != 0 {
				t.Errorf("%d requests sent", n)
			}
		})
	}
}

func TestApplePushPartialFailure(t *testing.T) {
	for _, mode := range []string{"text", "json"} {
		t.Run(mode, func(t *testing.T) {
			srv := pushCLIServer(t)
			srv.Handle("PATCH", "/v1/appInfoLocalizations/{id}", func(w http.ResponseWriter, r *http.Request) {
				w.Header().Set("Content-Type", "application/json")
				if r.PathValue("id") == "il-en" {
					w.WriteHeader(http.StatusConflict)
					_, _ = w.Write([]byte(`{"errors":[{"status":"409","code":"ENTITY_ERROR.ATTRIBUTE.INVALID","title":"An attribute value is invalid.","detail":"The subtitle can not be edited at this time."}]}`))
					return
				}
				_, _ = w.Write([]byte(`{"data":{"type":"appInfoLocalizations","id":"il-de"},"links":{"self":"x"}}`))
			})
			code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "push", pushDir("partial"), "-o", mode)
			if code != 1 || !strings.Contains(stderr, "applied 2 of 5 changes") {
				t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
			}
			assertRequests(t, apiRequests(srv, true), []string{
				"PATCH /v1/appInfoLocalizations/il-de",
				"PATCH /v1/appInfoLocalizations/il-en",
			})
			wantActions := []string{"update app-info de-DE name", "update app-info de-DE subtitle", "skip app-info en-US subtitle",
				"skip version de-DE description", "skip version en-US description"}
			if mode == "text" {
				// after the plan, stderr shows which changes were applied and which not
				_, result, _ := strings.Cut(stderr, "result (skip = not applied): 5 changes\n")
				for _, line := range wantActions {
					if !strings.Contains(result, "  "+line+"\n") {
						t.Errorf("stderr lacks %q:\n%s", line, stderr)
					}
				}
				if stdout != "" {
					t.Errorf("stdout = %q", stdout)
				}
				return
			}
			var got []string
			for _, c := range decodePlan(t, stdout).Changes {
				got = append(got, strings.Join([]string{c.Action, c.Scope, c.Locale, c.Field}, " "))
			}
			if !slices.Equal(got, wantActions) {
				t.Errorf("plan = %v, want %v", got, wantActions)
			}
		})
	}
}

func TestApplePushOnly(t *testing.T) {
	cases := []struct {
		only   string
		scopes []string
		absent []string // path fragments that must not be requested
	}{
		{"review", []string{"review"}, []string{"Localizations", "appInfos", "appScreenshot"}},
		{"texts,screenshots", []string{"app-info", "version", "screenshots"}, []string{"appStoreReviewDetail"}},
		{"screenshots", []string{"screenshots"}, []string{"appInfo", "appStoreReviewDetail"}},
	}
	for _, c := range cases {
		t.Run(c.only, func(t *testing.T) {
			srv := pushCLIServer(t)
			code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "push", pushDir("changes"), "--screenshots", pushShots(t), "--only", c.only, "-o", "json")
			if code != 0 {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			seen := map[string]bool{}
			for _, ch := range decodePlan(t, stdout).Changes {
				seen[ch.Scope] = true
			}
			for _, s := range c.scopes {
				if !seen[s] {
					t.Errorf("no %s change in %s", s, stdout)
				}
				delete(seen, s)
			}
			if len(seen) != 0 {
				t.Errorf("unexpected scopes %v in %s", seen, stdout)
			}
			for _, r := range apiRequests(srv, false) {
				for _, frag := range c.absent {
					if strings.Contains(r, frag) {
						t.Errorf("requested %s", r)
					}
				}
			}
		})
	}
	srv := pushCLIServer(t)
	code, _, stderr := runEnv(t, buildEnv(srv), false, "apple", "push", pushDir("changes"), "--only", "texts,images")
	if code != 2 || !strings.Contains(stderr, `"images"`) {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if n := len(srv.Requests()); n != 0 {
		t.Errorf("%d requests sent", n)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestApplePullWritesFiles(t *testing.T) {
	srv := pushCLIServer(t)
	dir := t.TempDir()
	mine := filepath.Join(dir, "en-US", "mine.md")
	if err := os.MkdirAll(filepath.Dir(mine), 0o755); err != nil {
		t.Fatal(err)
	}
	for p, c := range map[string]string{mine: "mine\n", filepath.Join(dir, "en-US", "name.txt"): "Old name\n"} {
		if err := os.WriteFile(p, []byte(c), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "pull", dir, "-o", "json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	// en-US: 10 fields, de-DE: 5, review: 7 of 8 (no password)
	if want := "{\n  \"locales\": [\n    \"de-DE\",\n    \"en-US\"\n  ],\n  \"files\": 22\n}\n"; stdout != want {
		t.Errorf("stdout = %s", stdout)
	}
	for p, want := range map[string]string{
		filepath.Join(dir, "en-US", "name.txt"):                       "Example App\n",
		filepath.Join(dir, "en-US", "release_notes.txt"):              "Bug fixes.\n",
		filepath.Join(dir, "de-DE", "keywords.txt"):                   "beispiel,app\n",
		filepath.Join(dir, "review_information", "first_name.txt"):    "Ada\n",
		filepath.Join(dir, "review_information", "demo_required.txt"): "true\n",
		mine: "mine\n",
	} {
		if got := readFile(t, p); got != want {
			t.Errorf("%s = %q, want %q", p, got, want)
		}
	}
	for _, p := range []string{filepath.Join(dir, "review_information", "demo_password.txt"), filepath.Join(dir, "de-DE", "marketing_url.txt")} {
		if _, err := os.Stat(p); !os.IsNotExist(err) {
			t.Errorf("%s written (err=%v)", p, err)
		}
	}
	if strings.Contains(stdout+stderr, "s3cret") {
		t.Errorf("password in output")
	}
	if got := apiRequests(srv, true); len(got) != 0 {
		t.Errorf("pull sent %v", got)
	}
}

func TestApplePullThenPushIsEmptyPlan(t *testing.T) {
	srv := pushCLIServer(t)
	dir := filepath.Join(t.TempDir(), "metadata")
	if code, _, stderr := runEnv(t, buildEnv(srv), false, "apple", "pull", dir); code != 0 {
		t.Fatalf("pull: code=%d stderr=%q", code, stderr)
	}
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "push", dir, "-o", "json")
	if code != 0 || stdout != "{\n  \"changes\": []\n}\n" || stderr != "" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	if got := apiRequests(srv, true); len(got) != 0 {
		t.Errorf("push sent %v", got)
	}
}

func TestAppleInitOffline(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "metadata")
	// no Apple env vars at all
	code, stdout, stderr := runEnv(t, nil, false, "apple", "init", dir, "--locale", "en-US", "--locale", "de-DE")
	if code != 0 || stderr != "" {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	golden.Assert(t, "apple_init.text", []byte(strings.ReplaceAll(stdout, tmp, "<tmp>")))
	for _, p := range []string{
		filepath.Join(dir, "en-US", "name.txt"),
		filepath.Join(dir, "de-DE", "release_notes.txt"),
		filepath.Join(dir, "review_information", "demo_password.txt"),
	} {
		if got := readFile(t, p); got != "" {
			t.Errorf("%s = %q", p, got)
		}
	}
	for _, p := range []string{filepath.Join(tmp, "screenshots", "en-US"), filepath.Join(tmp, "screenshots", "de-DE")} {
		if fi, err := os.Stat(p); err != nil || !fi.IsDir() {
			t.Errorf("%s: %v", p, err)
		}
	}

	code, _, stderr = runEnv(t, nil, false, "apple", "init", dir)
	if code != 2 || !strings.Contains(stderr, "--locale") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestAppleInitKeepsExisting(t *testing.T) {
	tmp := t.TempDir()
	dir := filepath.Join(tmp, "metadata")
	name := filepath.Join(dir, "en-US", "name.txt")
	if err := os.MkdirAll(filepath.Dir(name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(name, []byte("Keep me\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	shots := filepath.Join(tmp, "shots")
	code, stdout, stderr := runEnv(t, nil, false, "apple", "init", dir, "--locale", "en-US", "--locale", "fr-FR", "--screenshots", shots, "-o", "json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	var res struct{ Created, Existing []string }
	if err := json.Unmarshal([]byte(stdout), &res); err != nil {
		t.Fatal(err)
	}
	if got := readFile(t, name); got != "Keep me\n" {
		t.Errorf("name.txt = %q", got)
	}
	if !slices.Equal(res.Existing, []string{name}) || slices.Contains(res.Created, name) ||
		!slices.Contains(res.Created, filepath.Join(dir, "fr-FR", "name.txt")) || !slices.Contains(res.Created, filepath.Join(shots, "fr-FR")) {
		t.Errorf("result = %+v", res)
	}

	// a second run creates nothing
	code, stdout, _ = runEnv(t, nil, false, "apple", "init", dir, "--locale", "en-US", "--locale", "fr-FR", "--screenshots", shots, "-o", "json")
	if err := json.Unmarshal([]byte(stdout), &res); code != 0 || err != nil || len(res.Created) != 0 || len(res.Existing) != 2*10+8+2 {
		t.Errorf("code=%d result=%+v err=%v", code, res, err)
	}
}
