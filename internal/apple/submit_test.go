package apple

import (
	"encoding/json"
	"errors"
	"net/http"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
	"github.com/nikeee/store-cli/internal/testutil/golden"
)

// submitServer serves everything Submit touches. versions is the fixture of
// the app's versions, builds that of the builds.
func submitServer(t *testing.T, versions, builds string) *ascfake.Server {
	t.Helper()
	srv := listingServer(t, "app_appInfos.json", "appInfo_localizations_full.json", "version_localizations_en_only.json")
	srv.Handle("GET", "/v1/apps/{id}/appStoreVersions", ascfake.JSON(http.StatusOK, versions))
	srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, builds))
	srv.Handle("POST", "/v1/appStoreVersions", ascfake.JSON(http.StatusCreated, "version_created.json"))
	srv.Handle("PATCH", "/v1/appStoreVersions/{id}", ascfake.JSON(http.StatusOK, "version_created.json"))
	srv.Handle("PATCH", "/v1/appStoreVersions/{id}/relationships/build", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	srv.Handle("PATCH", "/v1/appStoreVersionLocalizations/{id}", ascfake.JSON(http.StatusOK, "versionLocalization_created.json"))
	srv.Handle("GET", "/v1/apps/{id}/reviewSubmissions", ascfake.JSON(http.StatusOK, "review_submissions_none.json"))
	srv.Handle("POST", "/v1/reviewSubmissions", ascfake.JSON(http.StatusCreated, "review_submission_created.json"))
	srv.Handle("POST", "/v1/reviewSubmissionItems", ascfake.JSON(http.StatusCreated, "review_submission_item_created.json"))
	srv.Handle("PATCH", "/v1/reviewSubmissions/{id}", ascfake.JSON(http.StatusOK, "review_submission_submitted.json"))
	return srv
}

func submitMutations(srv *ascfake.Server) []string {
	var out []string
	for _, r := range srv.Requests() {
		if r.Method != http.MethodGet {
			out = append(out, r.Method+" "+r.Path)
		}
	}
	return out
}

func submitInputs(t *testing.T) SubmitOptions {
	t.Helper()
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"metadata/en-US/description.txt": "Changed.",
		"notes/en-US.txt":                "Fixes",
	})
	return SubmitOptions{Version: "1.3.0", Platform: "ios", Build: "latest",
		MetadataDir: filepath.Join(root, "metadata"), Notes: map[string]string{"en-US": "Fixes"}}
}

func TestSubmitFullFlow(t *testing.T) {
	srv := submitServer(t, "app_appStoreVersions.json", "builds_valid.json")
	svc, stderr := uploadService(t, srv)
	o := submitInputs(t)
	o.ReleaseType = "AFTER_APPROVAL"
	res, err := svc.Submit(t.Context(), listingApp, o)
	if err != nil {
		t.Fatal(err)
	}
	if res.Version != "1.3.0" || res.Build != "45" || res.SubmissionID != submissionID || res.State != "WAITING_FOR_REVIEW" {
		t.Errorf("res = %+v", res)
	}
	want := []string{
		"PATCH /v1/appStoreVersions/ver-1",
		"PATCH /v1/appStoreVersions/ver-1/relationships/build",
		"PATCH /v1/appStoreVersionLocalizations/vl-en",
		"PATCH /v1/appStoreVersionLocalizations/vl-en",
		"POST /v1/reviewSubmissions",
		"POST /v1/reviewSubmissionItems",
		"PATCH /v1/reviewSubmissions/" + submissionID,
	}
	got := submitMutations(srv)
	for i := range got {
		got[i] = strings.Replace(got[i], "7d5f9a2e-3c41-4b8e-9f20-1a6b8c4d0130", "ver-1", 1)
	}
	if !slices.Equal(got, want) {
		t.Errorf("mutations =\n%v\nwant\n%v", got, want)
	}
	if lines := changeLines(res.Plan.Changes); !slices.Equal(lines, []string{
		"update version  release type", "update version  attach build", "update version en-US description",
		"update version en-US set release notes", "create submission  submit",
	}) {
		t.Errorf("plan = %v", lines)
	}
	if stderr.Len() != 0 {
		t.Errorf("stderr = %q", stderr)
	}
}

func TestSubmitCreatesVersion(t *testing.T) {
	srv := submitServer(t, "versions_live_only.json", "builds_valid.json")
	o := SubmitOptions{Version: "1.4.0", Platform: "ios", Build: "latest"}
	res, err := newService(t, srv).Submit(t.Context(), listingApp, o)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{
		"POST /v1/appStoreVersions",
		"PATCH /v1/appStoreVersions/8c1d2e3f-0000-4000-8000-000000000140/relationships/build",
		"POST /v1/reviewSubmissions",
		"POST /v1/reviewSubmissionItems",
		"PATCH /v1/reviewSubmissions/" + submissionID,
	}
	if got := submitMutations(srv); !slices.Equal(got, want) {
		t.Errorf("mutations = %v", got)
	}
	if lines := changeLines(res.Plan.Changes); lines[0] != "create version  create version" {
		t.Errorf("plan = %v", lines)
	}
}

func TestSubmitVersionNotEditable(t *testing.T) {
	srv := submitServer(t, "versions_pending_release.json", "builds_valid.json")
	_, err := newService(t, srv).Submit(t.Context(), listingApp, SubmitOptions{Version: "1.3.0", Platform: "ios"})
	if err == nil || clierr.ExitCode(err) != 1 || !strings.Contains(err.Error(), "PENDING_DEVELOPER_RELEASE") {
		t.Fatalf("err = %v", err)
	}
	if m := submitMutations(srv); len(m) != 0 {
		t.Errorf("mutations = %v", m)
	}
}

func TestSubmitFirstReleaseSkipsWhatsNew(t *testing.T) {
	srv := submitServer(t, "versions_two_editable.json", "builds_valid.json")
	svc, stderr := uploadService(t, srv)
	o := submitInputs(t)
	res, err := svc.Submit(t.Context(), listingApp, o)
	if err != nil {
		t.Fatal(err)
	}
	if res.SubmissionID != submissionID {
		t.Errorf("res = %+v", res)
	}
	if !strings.Contains(stderr.String(), "warning: skipping What's New: ") {
		t.Errorf("stderr = %q", stderr)
	}
	for _, l := range changeLines(res.Plan.Changes) {
		if strings.Contains(l, "release notes") {
			t.Errorf("plan has %q", l)
		}
	}
	// description patch only, no What's New patch
	n := 0
	for _, m := range submitMutations(srv) {
		if strings.HasPrefix(m, "PATCH /v1/appStoreVersionLocalizations/") {
			n++
		}
	}
	if n != 1 {
		t.Errorf("localization patches = %d", n)
	}
}

func TestSubmitNoBuild(t *testing.T) {
	srv := submitServer(t, "app_appStoreVersions.json", "builds_empty.json")
	_, err := newService(t, srv).Submit(t.Context(), listingApp, SubmitOptions{Version: "1.3.0", Platform: "ios", Build: "latest"})
	var ce *clierr.Error
	if !errors.As(err, &ce) || ce.Hint != "store apple build list --version 1.3.0" || clierr.ExitCode(err) != 1 {
		t.Fatalf("err = %v", err)
	}
	if m := submitMutations(srv); len(m) != 0 {
		t.Errorf("mutations = %v", m)
	}
}

func TestSubmitInvalidInputMakesNoRequest(t *testing.T) {
	srv := submitServer(t, "app_appStoreVersions.json", "builds_valid.json")
	root := t.TempDir()
	writeTree(t, root, map[string]string{"metadata/en-US/subtitle.txt": strings.Repeat("x", 31)})
	_, err := newService(t, srv).Submit(t.Context(), listingApp, SubmitOptions{Version: "1.3.0", Platform: "ios", MetadataDir: filepath.Join(root, "metadata")})
	if err == nil || clierr.ExitCode(err) != 1 {
		t.Fatalf("err = %v", err)
	}
	_, err = newService(t, srv).Submit(t.Context(), listingApp, SubmitOptions{Version: "1.3.0", Platform: "ios", Notes: map[string]string{"en-US": strings.Repeat("x", 4001)}})
	if err == nil || clierr.ExitCode(err) != 1 {
		t.Fatalf("err = %v", err)
	}
	if n := len(srv.Requests()); n != 0 {
		t.Errorf("requests = %d", n)
	}
}

func TestSubmitDryRun(t *testing.T) {
	cases := []struct{ name, versions, version string }{
		{"existing", "app_appStoreVersions.json", "1.3.0"},
		{"new", "versions_live_only.json", "1.4.0"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := submitServer(t, c.versions, "builds_valid.json")
			o := submitInputs(t)
			o.Version, o.DryRun, o.ReleaseType = c.version, true, "MANUAL"
			res, err := newService(t, srv).Submit(t.Context(), listingApp, o)
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range srv.Requests() {
				if r.Method != http.MethodGet {
					t.Errorf("dry run sent %s %s", r.Method, r.Path)
				}
			}
			if res.State != "dry-run" {
				t.Errorf("state = %q", res.State)
			}
			b, err := json.MarshalIndent(res, "", "  ")
			if err != nil {
				t.Fatal(err)
			}
			golden.Assert(t, "submit_dry_run_"+c.name+".json", append(b, '\n'))
		})
	}
}

func TestSubmitScheduledNeedsDate(t *testing.T) {
	_, err := newService(t, ascfake.New(t)).Submit(t.Context(), listingApp, SubmitOptions{Version: "1.3.0", Platform: "ios", ReleaseType: "SCHEDULED"})
	if err == nil || clierr.ExitCode(err) != 2 {
		t.Fatalf("err = %v", err)
	}
}

func TestSubmitFailureNamesDoneStepsAndSkipsRest(t *testing.T) {
	srv := submitServer(t, "versions_live_only.json", "builds_valid.json")
	srv.Handle("GET", "/v1/appStoreVersions/{id}/appStoreReviewDetail", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"data": nil, "links": map[string]any{"self": "x"}})
	})
	srv.Handle("PATCH", "/v1/appStoreVersionLocalizations/{id}", ascfake.JSON(http.StatusConflict, "error_409.json"))
	// the created version is listed once it exists, for push
	srv.Handle("GET", "/v1/apps/{id}/appStoreVersions", func(w http.ResponseWriter, r *http.Request) {
		if len(submitMutations(srv)) == 0 {
			ascfake.JSON(http.StatusOK, "versions_live_only.json")(w, r)
			return
		}
		ascfake.JSON(http.StatusOK, "app_appStoreVersions.json")(w, r)
	})
	root := t.TempDir()
	writeTree(t, root, map[string]string{
		"metadata/en-US/description.txt":                "Changed.",
		"metadata/review_information/first_name.txt":    "Ada",
		"metadata/review_information/demo_password.txt": "secret",
	})
	o := SubmitOptions{Version: "1.3.0", Platform: "ios", MetadataDir: filepath.Join(root, "metadata"), Notes: map[string]string{"en-US": "Fixes"}}
	res, err := newService(t, srv).Submit(t.Context(), listingApp, o)
	if err == nil {
		t.Fatal("no error")
	}
	if !strings.Contains(err.Error(), "done: create version 1.3.0, attach build 45; not done: push metadata, set release notes, submit; run the command again to continue") {
		t.Errorf("err = %v", err)
	}
	got := changeLines(res.Plan.Changes)
	want := []string{
		"create version  create version", "update version  attach build",
		"skip version en-US description", "skip review  contactFirstName", "skip review  demoAccountPassword",
		"skip version en-US set release notes", "skip submission  submit",
	}
	if !slices.Equal(got, want) {
		t.Errorf("plan = %v", got)
	}
	for _, c := range res.Plan.Changes {
		if c.New == "secret" {
			t.Errorf("secret in plan: %+v", c)
		}
	}
}
