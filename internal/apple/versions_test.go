package apple

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
)

var versionsApp = App{ID: "1234567890", BundleID: "com.example.app"}

func versionServer(t *testing.T, fixture string) *ascfake.Server {
	t.Helper()
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/apps/{id}/appStoreVersions", ascfake.JSON(http.StatusOK, fixture))
	return srv
}

func TestParseReleaseType(t *testing.T) {
	for in, want := range map[string]string{"manual": "MANUAL", "after-approval": "AFTER_APPROVAL", "scheduled": "SCHEDULED"} {
		if got, err := ParseReleaseType(in); err != nil || got != want {
			t.Errorf("ParseReleaseType(%q) = %q, %v", in, got, err)
		}
	}
	if _, err := ParseReleaseType("now"); err == nil || clierr.ExitCode(err) != 2 {
		t.Errorf("err = %v", err)
	}
}

func TestFindVersion(t *testing.T) {
	srv := versionServer(t, "app_appStoreVersions.json")
	svc := newService(t, srv)
	v, found, err := svc.FindVersion(t.Context(), versionsApp, "1.3.0", "ios")
	if err != nil || !found || v.ID != "7d5f9a2e-3c41-4b8e-9f20-1a6b8c4d0130" || v.Build == nil || v.Build.Build != "45" {
		t.Fatalf("v=%+v found=%v err=%v", v, found, err)
	}
	if _, found, _ := svc.FindVersion(t.Context(), versionsApp, "9.9", "ios"); found {
		t.Error("9.9 found")
	}
	q := srv.Requests()[0].Query
	if q.Get("filter[platform]") != "IOS" || q.Get("filter[versionString]") != "1.3.0" || q.Get("include") != "build" {
		t.Errorf("query = %s", q.Encode())
	}
}

func TestEditableVersionNone(t *testing.T) {
	_, err := newService(t, versionServer(t, "versions_live_only.json")).EditableVersion(t.Context(), versionsApp, "ios")
	var ce *clierr.Error
	if !errors.As(err, &ce) || ce.Hint != "store apple version create <version>" {
		t.Fatalf("err = %v", err)
	}
}

// A newer version that is not live yet is named: creating another version
// would not help.
func TestEditableVersionNoneNamesNewerVersion(t *testing.T) {
	for fixture, want := range map[string]struct{ msg, hint string }{
		"versions_in_review.json":       {"no editable version: 1.3.0 is in state WAITING_FOR_REVIEW", "store apple version cancel 1.3.0"},
		"versions_pending_release.json": {"no editable version: 1.3.0 is in state PENDING_DEVELOPER_RELEASE", ""},
	} {
		_, err := newService(t, versionServer(t, fixture)).EditableVersion(t.Context(), versionsApp, "ios")
		var ce *clierr.Error
		if !errors.As(err, &ce) {
			t.Fatalf("%s: err = %v", fixture, err)
		}
		if ce.Message != want.msg || ce.Hint != want.hint || ce.Code != "STATE_ERROR" {
			t.Errorf("%s: err = %v (hint %q, code %q)", fixture, err, ce.Hint, ce.Code)
		}
	}
}

func TestEditableVersionPicksEditable(t *testing.T) {
	v, err := newService(t, versionServer(t, "app_appStoreVersions.json")).EditableVersion(t.Context(), versionsApp, "ios")
	if err != nil || v.Version != "1.3.0" {
		t.Fatalf("v=%+v err=%v", v, err)
	}
}

func TestEditableVersionSeveral(t *testing.T) {
	_, err := newService(t, versionServer(t, "versions_two_editable.json")).EditableVersion(t.Context(), versionsApp, "ios")
	if err == nil || !strings.Contains(err.Error(), "1.3.0 (PREPARE_FOR_SUBMISSION)") || !strings.Contains(err.Error(), "1.4.0 (REJECTED)") {
		t.Fatalf("err = %v", err)
	}
}

func TestCurrentVersionFallsBackToLive(t *testing.T) {
	v, err := newService(t, versionServer(t, "versions_live_only.json")).CurrentVersion(t.Context(), versionsApp, "ios")
	if err != nil || v.Version != "1.2.0" {
		t.Fatalf("v=%+v err=%v", v, err)
	}
	v, err = newService(t, versionServer(t, "app_appStoreVersions.json")).CurrentVersion(t.Context(), versionsApp, "ios")
	if err != nil || v.Version != "1.3.0" {
		t.Fatalf("editable first: v=%+v err=%v", v, err)
	}
}

func TestHasReleasedVersion(t *testing.T) {
	for fixture, want := range map[string]bool{"app_appStoreVersions.json": true, "versions_live_only.json": true, "versions_two_editable.json": false} {
		got, err := newService(t, versionServer(t, fixture)).HasReleasedVersion(t.Context(), versionsApp, "ios")
		if err != nil || got != want {
			t.Errorf("%s: got %v, %v", fixture, got, err)
		}
	}
}

func TestCreateVersionExists(t *testing.T) {
	srv := versionServer(t, "app_appStoreVersions.json")
	_, err := newService(t, srv).CreateVersion(t.Context(), versionsApp, CreateVersionOptions{Version: "1.3.0", Platform: "ios"})
	if err == nil || !strings.Contains(err.Error(), "version 1.3.0 already exists") || clierr.ExitCode(err) != 1 {
		t.Fatalf("err = %v", err)
	}
	for _, r := range srv.Requests() {
		if r.Method != http.MethodGet {
			t.Errorf("sent %s", r.Method)
		}
	}
}

func TestCreateScheduledNeedsDate(t *testing.T) {
	srv := ascfake.New(t)
	_, err := newService(t, srv).CreateVersion(t.Context(), versionsApp, CreateVersionOptions{Version: "1.4.0", Platform: "ios", ReleaseType: "SCHEDULED"})
	if err == nil || clierr.ExitCode(err) != 2 || !strings.Contains(err.Error(), "--release-date") {
		t.Fatalf("err = %v exit=%d", err, clierr.ExitCode(err))
	}
}

func TestCheckReleaseDate(t *testing.T) {
	date := time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC)
	for _, c := range []struct {
		releaseType string
		date        *time.Time
		wantErr     string
	}{
		{"", nil, ""},
		{"MANUAL", nil, ""},
		{"SCHEDULED", &date, ""},
		{"SCHEDULED", nil, "--release scheduled needs --release-date"},
		{"", &date, "--release-date needs --release scheduled"},
		{"AFTER_APPROVAL", &date, "--release-date needs --release scheduled"},
	} {
		err := CheckReleaseDate(c.releaseType, c.date)
		if c.wantErr == "" {
			if err != nil {
				t.Errorf("%q, %v: err = %v", c.releaseType, c.date, err)
			}
			continue
		}
		if err == nil || clierr.ExitCode(err) != 2 || err.Error() != c.wantErr {
			t.Errorf("%q, %v: err = %v, want %q", c.releaseType, c.date, err, c.wantErr)
		}
	}
}

func TestCreateVersion(t *testing.T) {
	srv := versionServer(t, "versions_live_only.json")
	srv.Handle("POST", "/v1/appStoreVersions", ascfake.JSON(http.StatusCreated, "version_created.json"))
	date := time.Date(2026, 12, 1, 9, 0, 0, 0, time.UTC)
	copyright := "2026 Example GmbH"
	v, err := newService(t, srv).CreateVersion(t.Context(), versionsApp, CreateVersionOptions{
		Version: "1.4.0", Platform: "ios", ReleaseType: "SCHEDULED", ReleaseDate: &date, Copyright: &copyright,
	})
	if err != nil || v.Version != "1.4.0" || v.State != "PREPARE_FOR_SUBMISSION" || v.Platform != "ios" {
		t.Fatalf("v=%+v err=%v", v, err)
	}
	reqs := srv.Requests()
	var body struct {
		Data struct {
			Attributes    map[string]any
			Relationships struct {
				App struct{ Data struct{ ID string } }
			}
		}
	}
	if err := json.Unmarshal(reqs[len(reqs)-1].Body, &body); err != nil {
		t.Fatal(err)
	}
	a := body.Data.Attributes
	if a["platform"] != "IOS" || a["versionString"] != "1.4.0" || a["releaseType"] != "SCHEDULED" ||
		a["earliestReleaseDate"] != "2026-12-01T09:00:00Z" || a["copyright"] != copyright || body.Data.Relationships.App.Data.ID != "1234567890" {
		t.Errorf("body = %s", reqs[len(reqs)-1].Body)
	}
}

func TestCreateVersionDryRun(t *testing.T) {
	srv := versionServer(t, "versions_live_only.json")
	v, err := newService(t, srv).CreateVersion(t.Context(), versionsApp, CreateVersionOptions{Version: "1.4.0", Platform: "ios", DryRun: true})
	if err != nil || v.Version != "1.4.0" {
		t.Fatalf("v=%+v err=%v", v, err)
	}
	for _, r := range srv.Requests() {
		if r.Method != http.MethodGet {
			t.Errorf("sent %s", r.Method)
		}
	}
}

func editableVersion() Version {
	return Version{ID: "v1", Version: "1.3.0", State: "PREPARE_FOR_SUBMISSION"}
}

func TestSetBuildRequiresValid(t *testing.T) {
	srv := ascfake.New(t)
	_, err := newService(t, srv).SetVersionBuild(t.Context(), editableVersion(), Build{ID: "b1", Build: "45", State: "processing"})
	if err == nil || !strings.Contains(err.Error(), "build 45 is processing") || clierr.ExitCode(err) != 1 {
		t.Fatalf("err = %v", err)
	}
}

func TestSetBuildRequiresEditable(t *testing.T) {
	srv := ascfake.New(t)
	v := editableVersion()
	v.State = "WAITING_FOR_REVIEW"
	_, err := newService(t, srv).SetVersionBuild(t.Context(), v, Build{ID: "b1", Build: "45", State: "valid"})
	if err == nil || !strings.Contains(err.Error(), "state WAITING_FOR_REVIEW") {
		t.Fatalf("err = %v", err)
	}
}

func TestSetVersionBuild(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("PATCH", "/v1/appStoreVersions/{id}/relationships/build", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusNoContent) })
	v, err := newService(t, srv).SetVersionBuild(t.Context(), editableVersion(), Build{ID: "b1", Build: "45", State: "valid"})
	if err != nil || v.Build == nil || v.Build.ID != "b1" || v.Build.Build != "45" {
		t.Fatalf("v=%+v err=%v", v, err)
	}
	r := srv.Requests()[0]
	if r.Path != "/v1/appStoreVersions/v1/relationships/build" || !strings.Contains(string(r.Body), `"type":"builds"`) || !strings.Contains(string(r.Body), `"id":"b1"`) {
		t.Errorf("request = %s %s", r.Path, r.Body)
	}
}

// The API has no sort for versions, so the list is sorted by creation,
// newest first, before it is cut to the limit.
func TestListVersionsNewestFirst(t *testing.T) {
	srv := ascfake.New(t)
	version := func(id, v, created string) string {
		return fmt.Sprintf(`{"type":"appStoreVersions","id":%q,"attributes":{"platform":"IOS","versionString":%q,"appVersionState":"READY_FOR_DISTRIBUTION","createdDate":%q}}`, id, v, created)
	}
	srv.Handle("GET", "/v1/apps/{id}/appStoreVersions", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") != "" {
			_, _ = fmt.Fprintf(w, `{"data":[%s],"links":{"self":"x"}}`, version("v3", "1.2.0", "2026-09-01T08:30:00Z"))
			return
		}
		_, _ = fmt.Fprintf(w, `{"data":[%s,%s],"links":{"self":"x","next":"%s/v1/apps/%s/appStoreVersions?cursor=2"}}`,
			version("v1", "1.0.0", "2026-01-01T08:30:00Z"), version("v2", "1.1.0", "2026-06-15T11:00:00Z"), srv.URL, versionsApp.ID)
	})
	vs, err := newService(t, srv).ListVersions(t.Context(), versionsApp, "ios", 2)
	if err != nil || len(vs) != 2 || vs[0].Version != "1.2.0" || vs[1].Version != "1.1.0" {
		t.Fatalf("versions = %+v, err = %v", vs, err)
	}
}
