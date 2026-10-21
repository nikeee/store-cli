package apple

import (
	"context"
	"io"
	"net/http"
	"reflect"
	"testing"
	"time"

	"github.com/nikeee/store-cli/internal/apple/ascclient"
	"github.com/nikeee/store-cli/internal/output"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
)

// newService returns a Service talking to srv, with instant polling.
func newService(t *testing.T, srv *ascfake.Server) *Service {
	t.Helper()
	c, err := ascclient.New(ascfake.Credentials(), &http.Client{}, srv.URL, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	r := output.NewPrinter(output.ModeText, io.Discard, io.Discard, false)
	return NewService(c, r, Poll{Interval: time.Second, Sleep: func(ctx context.Context, _ time.Duration) error { return ctx.Err() }}, time.Now)
}

func TestAPIPlatform(t *testing.T) {
	for flag, want := range map[string]string{"ios": "IOS", "macos": "MAC_OS", "tvos": "TV_OS", "visionos": "VISION_OS"} {
		got, ok := APIPlatform(flag)
		if !ok || string(got) != want {
			t.Errorf("APIPlatform(%q) = %q, %v", flag, got, ok)
		}
	}
	if _, ok := APIPlatform("android"); ok {
		t.Error("android accepted")
	}
}

func TestCheckAuth(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/apps", ascfake.JSON(http.StatusOK, "apps_one.json"))
	if err := newService(t, srv).CheckAuth(t.Context()); err != nil {
		t.Fatal(err)
	}
	if q := srv.Requests()[0].Query; q.Encode() != "limit=1" {
		t.Errorf("query = %s", q.Encode())
	}
}

func TestListApps(t *testing.T) {
	srv := ascfake.New(t)
	page1, page2 := ascfake.JSON(http.StatusOK, "apps_page1.json"), ascfake.JSON(http.StatusOK, "apps_page2.json")
	srv.Handle("GET", "/v1/apps", func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Query().Get("cursor") != "" {
			page2(w, r)
			return
		}
		page1(w, r)
	})
	apps, err := newService(t, srv).ListApps(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	want := []App{
		{ID: "1234567890", BundleID: "com.example.app", Name: "Example", SKU: "EXAMPLE1", PrimaryLocale: "en-US"},
		{ID: "1234567891", BundleID: "com.example.pro", Name: "Example Pro", SKU: "EXAMPLE2", PrimaryLocale: "de-DE"},
		{ID: "1234567892", BundleID: "de.example.beispiel", Name: "Beispiel", SKU: "BEISPIEL", PrimaryLocale: "de-DE"},
	}
	if !reflect.DeepEqual(apps, want) {
		t.Errorf("apps = %+v", apps)
	}
	if q := srv.Requests()[0].Query; q.Get("limit") != "200" {
		t.Errorf("first page query = %s", q.Encode())
	}
}

func TestShowApp(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/apps/{id}/appStoreVersions", ascfake.JSON(http.StatusOK, "app_appStoreVersions.json"))
	srv.Handle("GET", "/v1/apps/{id}/appInfos", ascfake.JSON(http.StatusOK, "app_appInfos.json"))
	srv.Handle("GET", "/v1/appInfos/{id}/appInfoLocalizations", ascfake.JSON(http.StatusOK, "appInfo_appInfoLocalizations.json"))
	app := App{ID: "1234567890", BundleID: "com.example.app"}

	// tvos only shows in the filter; the fake ignores it and serves IOS versions.
	got, err := newService(t, srv).ShowApp(t.Context(), app, "tvos")
	if err != nil {
		t.Fatal(err)
	}
	release := time.Date(2026, 11, 2, 16, 0, 0, 0, time.UTC)
	want := AppDetails{
		App: app,
		Versions: []Version{
			{ID: "7d5f9a2e-3c41-4b8e-9f20-1a6b8c4d0130", Platform: "ios", Version: "1.3.0", State: "PREPARE_FOR_SUBMISSION", ReleaseType: "SCHEDULED",
				EarliestReleaseDate: &release, Build: &VersionBuild{ID: "4f3c2a10-6c1e-4d4b-9a51-2f0a7d9e8b45", Build: "45"}},
			{ID: "2b8e6c1d-9a07-4f3e-8d52-6e0f1b7a0120", Platform: "ios", Version: "1.2.0", State: "READY_FOR_DISTRIBUTION", ReleaseType: "MANUAL",
				Build: &VersionBuild{ID: "9b8e1f22-0d7a-4c3e-8f61-5a2b3c4d5e42", Build: "42"}},
			{ID: "c3a1d7f4-5e28-4b6a-a9c0-8f2d4e6b0110", Platform: "ios", Version: "1.1.0", State: "REPLACED_WITH_NEW_VERSION", ReleaseType: "AFTER_APPROVAL"},
		},
		Locales: []string{"de-DE", "en-US"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got  %+v\nwant %+v", got, want)
	}

	reqs := srv.Requests()
	if len(reqs) != 3 {
		t.Fatalf("%d requests", len(reqs))
	}
	if q := reqs[0].Query; reqs[0].Path != "/v1/apps/1234567890/appStoreVersions" ||
		q.Get("filter[platform]") != "TV_OS" || q.Get("include") != "build" || q.Get("limit") != "5" {
		t.Errorf("versions request = %s?%s", reqs[0].Path, q.Encode())
	}
	// The editable app info, not the live one.
	if reqs[2].Path != "/v1/appInfos/b7d3f1a0-edit-4c2d-9b4e-000000000002/appInfoLocalizations" {
		t.Errorf("localizations request = %s", reqs[2].Path)
	}
}
