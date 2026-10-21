package apple

import (
	"errors"
	"net/http"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
)

func releaseNotesServer(t *testing.T, versions string) *ascfake.Server {
	t.Helper()
	srv := listingServer(t, "app_appInfos.json", "appInfo_localizations_full.json", "version_localizations_en_only.json")
	srv.Handle("GET", "/v1/apps/{id}/appStoreVersions", ascfake.JSON(http.StatusOK, versions))
	srv.Handle("PATCH", "/v1/appStoreVersionLocalizations/{id}", ascfake.JSON(http.StatusOK, "versionLocalization_created.json"))
	return srv
}

func TestSetReleaseNotes(t *testing.T) {
	srv := releaseNotesServer(t, "app_appStoreVersions.json")
	got, err := newService(t, srv).SetReleaseNotes(t.Context(), listingApp, listingVersion, map[string]string{"en-US": "Fixes"})
	if err != nil || len(got) != 1 || got[0] != (ReleaseNote{Locale: "en-US", WhatsNew: "Fixes"}) {
		t.Fatalf("got=%v err=%v", got, err)
	}
	p := requestsOf(srv, "PATCH")
	if len(p) != 1 || string(p[0].Body) != `{"data":{"type":"appStoreVersionLocalizations","id":"vl-en","attributes":{"whatsNew":"Fixes"}}}` {
		t.Errorf("patches = %v", p)
	}
}

func TestReleaseNotesBeforeFirstRelease(t *testing.T) {
	srv := releaseNotesServer(t, "versions_two_editable.json")
	_, err := newService(t, srv).SetReleaseNotes(t.Context(), listingApp, listingVersion, map[string]string{"en-US": "Fixes"})
	var ce *clierr.Error
	if !errors.As(err, &ce) || ce.Code != "NOT_YET_RELEASED" || ce.Message != "App Store Connect accepts What's New only after the first release" {
		t.Fatalf("err = %v", err)
	}
	if len(requestsOf(srv, "PATCH")) != 0 {
		t.Error("sent a change")
	}
}

func TestReleaseNotesMissingLocale(t *testing.T) {
	srv := releaseNotesServer(t, "app_appStoreVersions.json")
	_, err := newService(t, srv).SetReleaseNotes(t.Context(), listingApp, listingVersion, map[string]string{"en-US": "a", "de-DE": "b"})
	var ce *clierr.Error
	if !errors.As(err, &ce) || ce.Hint != "store apple listing locale add de-DE" {
		t.Fatalf("err = %v", err)
	}
	if len(requestsOf(srv, "PATCH")) != 0 {
		t.Error("partial write")
	}
}

func TestReleaseNotesTooLong(t *testing.T) {
	srv := releaseNotesServer(t, "app_appStoreVersions.json")
	_, err := newService(t, srv).SetReleaseNotes(t.Context(), listingApp, listingVersion, map[string]string{"en-US": strings.Repeat("x", 4001)})
	if err == nil || clierr.ExitCode(err) != 1 || !strings.Contains(err.Error(), "limit 4000") {
		t.Fatalf("err = %v", err)
	}
}
