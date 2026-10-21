package ascclient_test

import (
	"errors"
	"net/http"
	"testing"
	"time"

	"github.com/nikeee/store-cli/internal/apple/ascclient"
	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
)

func newClient(t *testing.T, srv *ascfake.Server, now func() time.Time) *ascclient.Client {
	t.Helper()
	c, err := ascclient.New(ascfake.Credentials(), &http.Client{}, srv.URL, now)
	if err != nil {
		t.Fatal(err)
	}
	return c
}

func TestResolveApp(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/apps", ascfake.JSON(http.StatusOK, "apps_one.json"))
	c := newClient(t, srv, time.Now)

	app, err := c.ResolveApp(t.Context(), "com.example.app")
	if err != nil || app.ID != "1234567890" {
		t.Fatalf("app=%+v err=%v", app, err)
	}
	if q := srv.Requests()[0].Query; q.Get("filter[bundleId]") != "com.example.app" {
		t.Errorf("query = %v", q)
	}
	// The filter result is checked for an exact match.
	if _, err := c.ResolveApp(t.Context(), "com.example"); err == nil {
		t.Error("prefix of a bundle ID resolved to an app")
	}
}

func TestResolveAppNotFound(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/apps", ascfake.JSON(http.StatusOK, "apps_empty.json"))
	c := newClient(t, srv, time.Now)

	_, err := c.ResolveApp(t.Context(), "com.example.missing")
	var ce *clierr.Error
	if !errors.As(err, &ce) {
		t.Fatalf("err = %v", err)
	}
	if ce.Message != `no app with bundle ID "com.example.missing" in App Store Connect` || ce.Hint != "store apple app list" || clierr.ExitCode(err) != 1 {
		t.Errorf("err = %+v", ce)
	}
}
