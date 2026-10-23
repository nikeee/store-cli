package apple

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nikeee/store-cli/internal/apple/ascclient"
	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/output"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
)

var testApp = App{ID: "1234567890", BundleID: "com.example.app"}

// clockService returns a Service whose clock advances by the slept time, so
// timeouts need no real waiting. It counts sleeps in sleeps.
func clockService(t *testing.T, srv *ascfake.Server, sleeps *atomic.Int32) *Service {
	t.Helper()
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	clock := func() time.Time { return now }
	c, err := ascclient.New(ascfake.Credentials(), &http.Client{}, srv.URL, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	r := output.NewPrinter(output.ModeText, io.Discard, io.Discard, false)
	sleep := func(_ context.Context, d time.Duration) error {
		sleeps.Add(1)
		now = now.Add(d)
		return nil
	}
	return NewService(c, r, Poll{Interval: time.Minute, Sleep: sleep}, clock)
}

func TestListBuilds(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, "builds_list.json"))
	builds, err := newService(t, srv).ListBuilds(t.Context(), testApp, BuildFilter{Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	if len(builds) != 3 {
		t.Fatalf("builds = %+v", builds)
	}
	b := builds[1]
	if b.Build != "44" || b.Version != "1.3.0" || b.Platform != "ios" || b.State != "valid" ||
		b.Expired || b.UsesNonExemptEncryption == nil || *b.UsesNonExemptEncryption ||
		!b.UploadedAt.Equal(time.Date(2026, 10, 18, 14, 5, 12, 0, time.UTC)) {
		t.Errorf("build = %+v", b)
	}
	if builds[0].UsesNonExemptEncryption != nil || builds[2].Version != "1.2.0" {
		t.Errorf("builds = %+v", builds)
	}
}

func TestListBuildsFilters(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, "builds_list.json"))
	_, err := newService(t, srv).ListBuilds(t.Context(), testApp,
		BuildFilter{Version: "1.3.0", State: "valid", Limit: 5, Platform: "macos"})
	if err != nil {
		t.Fatal(err)
	}
	q := srv.Requests()[0].Query
	for k, want := range map[string]string{
		"filter[app]":                        "1234567890",
		"filter[preReleaseVersion.version]":  "1.3.0",
		"filter[preReleaseVersion.platform]": "MAC_OS",
		"filter[processingState]":            "VALID",
		"sort":                               "-uploadedDate",
		"include":                            "preReleaseVersion",
		"limit":                              "5",
	} {
		if got := q.Get(k); got != want {
			t.Errorf("%s = %q, want %q (query %s)", k, got, want, q.Encode())
		}
	}
}

func TestListBuildsInvalidState(t *testing.T) {
	_, err := newService(t, ascfake.New(t)).ListBuilds(t.Context(), testApp, BuildFilter{State: "done"})
	if clierr.ExitCode(err) != 2 || !strings.Contains(err.Error(), "--state") {
		t.Fatalf("err = %v", err)
	}
}

func TestResolveBuildLatest(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, "builds_list.json"))
	b, err := newService(t, srv).ResolveBuild(t.Context(), testApp, "latest", "1.3.0", "ios")
	if err != nil {
		t.Fatal(err)
	}
	if b.Build != "45" || b.ID != "b3f1c2d4-0000-4a11-9c01-000000000045" {
		t.Errorf("build = %+v", b)
	}
	q := srv.Requests()[0].Query
	if q.Get("sort") != "-uploadedDate" || q.Get("limit") != "1" || q.Get("filter[preReleaseVersion.version]") != "1.3.0" || q.Get("filter[version]") != "" {
		t.Errorf("query = %s", q.Encode())
	}
}

func TestResolveBuildByNumber(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, "builds_valid.json"))
	b, err := newService(t, srv).ResolveBuild(t.Context(), testApp, "45", "", "ios")
	if err != nil || b.Build != "45" || b.State != "valid" {
		t.Fatalf("b=%+v err=%v", b, err)
	}
	if q := srv.Requests()[0].Query; q.Get("filter[version]") != "45" || q.Get("filter[app]") != "1234567890" {
		t.Errorf("query = %s", q.Encode())
	}
}

func TestResolveBuildAmbiguous(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, "builds_ambiguous.json"))
	_, err := newService(t, srv).ResolveBuild(t.Context(), testApp, "42", "", "ios")
	if clierr.ExitCode(err) != 2 || err.Error() != "build 42 exists for versions 1.0, 1.1; pass --version" {
		t.Fatalf("err = %v (exit %d)", err, clierr.ExitCode(err))
	}
}

// A build number on the second page still counts: versions of both pages.
func TestResolveBuildReadsAllPages(t *testing.T) {
	srv := ascfake.New(t)
	page := func(id, pre, version, links string) string {
		return fmt.Sprintf(`{"data":[{"type":"builds","id":%q,"attributes":{"version":"42","uploadedDate":"2026-10-20T09:30:00Z","processingState":"VALID"},`+
			`"relationships":{"preReleaseVersion":{"data":{"type":"preReleaseVersions","id":%q}}}}],`+
			`"included":[{"type":"preReleaseVersions","id":%q,"attributes":{"version":%q,"platform":"IOS"}}],"links":%s}`, id, pre, pre, version, links)
	}
	srv.Handle("GET", "/v1/builds", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if r.URL.Query().Get("cursor") != "" {
			_, _ = io.WriteString(w, page("b-b", "pre-10", "1.0", `{"self":"x"}`))
			return
		}
		_, _ = io.WriteString(w, page("b-a", "pre-11", "1.1", fmt.Sprintf(`{"self":"x","next":"%s/v1/builds?cursor=2"}`, srv.URL)))
	})
	_, err := newService(t, srv).ResolveBuild(t.Context(), testApp, "42", "", "ios")
	if err == nil || err.Error() != "build 42 exists for versions 1.0, 1.1; pass --version" {
		t.Fatalf("err = %v", err)
	}
}

func TestResolveBuildNotFound(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, "builds_empty.json"))
	_, err := newService(t, srv).ResolveBuild(t.Context(), testApp, "42", "", "ios")
	var ce *clierr.Error
	if !errors.As(err, &ce) || clierr.ExitCode(err) != 1 || ce.Hint != "store apple build list" || !strings.Contains(ce.Message, "build 42") {
		t.Fatalf("err = %#v", err)
	}
}

func TestWaitForBuildProcessed(t *testing.T) {
	srv := ascfake.New(t)
	var calls atomic.Int32
	empty, processing, valid := ascfake.JSON(http.StatusOK, "builds_empty.json"),
		ascfake.JSON(http.StatusOK, "builds_processing.json"), ascfake.JSON(http.StatusOK, "builds_valid.json")
	srv.Handle("GET", "/v1/builds", func(w http.ResponseWriter, r *http.Request) {
		switch calls.Add(1) {
		case 1:
			empty(w, r)
		case 2:
			processing(w, r)
		default:
			valid(w, r)
		}
	})
	var sleeps atomic.Int32
	b, err := clockService(t, srv, &sleeps).WaitForBuild(t.Context(), testApp, "45", "", "ios", BuildProcessed, 30*time.Minute)
	if err != nil {
		t.Fatal(err)
	}
	if b.State != "valid" || calls.Load() != 3 || sleeps.Load() != 2 {
		t.Errorf("state=%s calls=%d sleeps=%d", b.State, calls.Load(), sleeps.Load())
	}
}

func TestWaitForBuildExists(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, "builds_processing.json"))
	var sleeps atomic.Int32
	b, err := clockService(t, srv, &sleeps).WaitForBuild(t.Context(), testApp, "45", "", "ios", BuildExists, time.Hour)
	if err != nil || b.State != "processing" || sleeps.Load() != 0 {
		t.Fatalf("b=%+v err=%v sleeps=%d", b, err, sleeps.Load())
	}
}

func TestWaitForBuildFailed(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, "builds_failed.json"))
	var sleeps atomic.Int32
	_, err := clockService(t, srv, &sleeps).WaitForBuild(t.Context(), testApp, "45", "", "ios", BuildProcessed, time.Hour)
	if err == nil || clierr.ExitCode(err) != 1 || !strings.Contains(err.Error(), "build 45") || !strings.Contains(err.Error(), "failed") {
		t.Fatalf("err = %v", err)
	}
}

func TestWaitForBuildTimeout(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, "builds_processing.json"))
	var sleeps atomic.Int32
	_, err := clockService(t, srv, &sleeps).WaitForBuild(t.Context(), testApp, "45", "", "ios", BuildProcessed, 30*time.Minute)
	want := "timed out after 30m0s waiting for build 45 to be processed"
	if err == nil || err.Error() != want || clierr.ExitCode(err) != 1 {
		t.Fatalf("err = %v", err)
	}
	if sleeps.Load() != 30 {
		t.Errorf("sleeps = %d, want 30 (Poll.Interval is 1m)", sleeps.Load())
	}
}

// The last poll sleeps only what is left of the timeout.
func TestWaitForBuildLastSleepIsCapped(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, "builds_processing.json"))
	var sleeps atomic.Int32
	svc := clockService(t, srv, &sleeps)
	start := svc.now()
	_, err := svc.WaitForBuild(t.Context(), testApp, "45", "", "ios", BuildProcessed, 90*time.Second)
	if err == nil || clierr.ExitCode(err) != 1 {
		t.Fatalf("err = %v", err)
	}
	if waited := svc.now().Sub(start); waited != 90*time.Second {
		t.Errorf("waited %s, want 1m30s", waited)
	}
}

func TestSetEncryptionPatchBody(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("PATCH", "/v1/builds/{id}", ascfake.JSON(http.StatusOK, "build_updated.json"))
	in := Build{ID: "b3f1c2d4-0000-4a11-9c01-000000000044", Version: "1.3.0", Build: "44", Platform: "ios", State: "valid"}
	out, err := newService(t, srv).SetEncryption(t.Context(), in, false)
	if err != nil {
		t.Fatal(err)
	}
	req := srv.Requests()[0]
	var body struct {
		Data struct {
			Type, ID   string
			Attributes map[string]any
		}
	}
	if err := json.Unmarshal(req.Body, &body); err != nil {
		t.Fatal(err)
	}
	if req.Path != "/v1/builds/"+in.ID || body.Data.Type != "builds" || body.Data.ID != in.ID {
		t.Errorf("request = %s %s", req.Path, req.Body)
	}
	if got, _ := json.Marshal(body.Data.Attributes); string(got) != `{"usesNonExemptEncryption":false}` {
		t.Errorf("attributes = %s", got)
	}
	if out.Version != "1.3.0" || out.UsesNonExemptEncryption == nil || *out.UsesNonExemptEncryption {
		t.Errorf("out = %+v", out)
	}
}

func TestExpireBuildPatchBody(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("PATCH", "/v1/builds/{id}", ascfake.JSON(http.StatusOK, "build_expired.json"))
	out, err := newService(t, srv).ExpireBuild(t.Context(), Build{ID: "b3f1c2d4-0000-4a11-9c01-000000000044", Version: "1.3.0"})
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(srv.Requests()[0].Body), `"attributes":{"expired":true}`) || !out.Expired || out.Version != "1.3.0" {
		t.Errorf("body=%s out=%+v", srv.Requests()[0].Body, out)
	}
}

func TestResolveBuildFiltersPlatform(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, "builds_valid.json"))
	svc := newService(t, srv)
	if _, err := svc.ResolveBuild(t.Context(), testApp, "45", "", "macos"); err != nil {
		t.Fatal(err)
	}
	if _, err := svc.ResolveBuild(t.Context(), testApp, "latest", "", "ios"); err != nil {
		t.Fatal(err)
	}
	for i, want := range []string{"MAC_OS", "IOS"} {
		if got := srv.Requests()[i].Query.Get("filter[preReleaseVersion.platform]"); got != want {
			t.Errorf("request %d platform filter = %q, want %q", i, got, want)
		}
	}
	if _, err := svc.ResolveBuild(t.Context(), testApp, "45", "", ""); err == nil {
		t.Error("empty platform accepted")
	}
}

// The same build number under iOS 1.2 and macOS 1.3 is not ambiguous once
// the platform is given: the API only returns the requested platform.
func TestResolveBuildSharedNumberAcrossPlatforms(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/builds", func(w http.ResponseWriter, r *http.Request) {
		fixture := "builds_valid.json"
		if r.URL.Query().Get("filter[preReleaseVersion.platform]") == "MAC_OS" {
			fixture = "builds_mac.json"
		}
		ascfake.JSON(http.StatusOK, fixture)(w, r)
	})
	svc := newService(t, srv)
	mac, err := svc.ResolveBuild(t.Context(), testApp, "45", "", "macos")
	if err != nil || mac.Platform != "macos" || mac.Version != "1.4.0" {
		t.Fatalf("mac=%+v err=%v", mac, err)
	}
}

// WaitForBuild looks for the build on the given platform only.
func TestWaitForBuildFiltersPlatform(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, "builds_valid.json"))
	var sleeps atomic.Int32
	if _, err := clockService(t, srv, &sleeps).WaitForBuild(t.Context(), testApp, "45", "1.3.0", "macos", BuildExists, time.Minute); err != nil {
		t.Fatal(err)
	}
	q := srv.Requests()[0].Query
	if q.Get("filter[preReleaseVersion.platform]") != "MAC_OS" || q.Get("filter[preReleaseVersion.version]") != "1.3.0" || q.Get("filter[version]") != "45" {
		t.Errorf("query = %s", q.Encode())
	}
}
