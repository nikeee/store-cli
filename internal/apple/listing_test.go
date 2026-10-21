package apple

import (
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/metadata"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
)

var (
	listingApp     = App{ID: "1234567890", BundleID: "com.example.app", Name: "Example", PrimaryLocale: "en-US"}
	listingVersion = Version{ID: "ver-1", Version: "1.3.0", Platform: "ios", State: "PREPARE_FOR_SUBMISSION"}
)

func listingServer(t *testing.T, infos, infoLocs, verLocs string) *ascfake.Server {
	t.Helper()
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/apps/{id}/appInfos", ascfake.JSON(http.StatusOK, infos))
	srv.Handle("GET", "/v1/appInfos/{id}/appInfoLocalizations", ascfake.JSON(http.StatusOK, infoLocs))
	srv.Handle("GET", "/v1/appStoreVersions/{id}/appStoreVersionLocalizations", ascfake.JSON(http.StatusOK, verLocs))
	return srv
}

func requestsOf(srv *ascfake.Server, method string) []ascfake.Request {
	var out []ascfake.Request
	for _, r := range srv.Requests() {
		if r.Method == method {
			out = append(out, r)
		}
	}
	return out
}

func TestListingTextsMergesLevels(t *testing.T) {
	srv := listingServer(t, "app_appInfos.json", "appInfo_localizations_full.json", "version_localizations.json")
	got, err := newService(t, srv).ListingTexts(t.Context(), listingApp, listingVersion)
	if err != nil {
		t.Fatal(err)
	}
	en := got["en-US"]
	want := metadata.Texts{
		"name": "Example App", "subtitle": "Do things", "privacyPolicyUrl": "https://example.com/privacy",
		"privacyChoicesUrl": "https://example.com/choices", "description": "An example app.", "keywords": "example,app",
		"promotionalText": "New!", "marketingUrl": "https://example.com", "supportUrl": "https://example.com/support",
		"whatsNew": "Bug fixes.",
	}
	if len(en) != len(want) {
		t.Fatalf("en-US = %v", en)
	}
	for k, v := range want {
		if en[k] != v {
			t.Errorf("%s = %q, want %q", k, en[k], v)
		}
	}
	if len(got) != 2 || got["de-DE"]["name"] != "Beispiel App" || got["de-DE"]["description"] != "Eine Beispiel-App." {
		t.Errorf("got = %v", got)
	}
}

func TestListingMappingsCoverAppleFields(t *testing.T) {
	for _, f := range metadata.AppleFields {
		_, inInfo := infoFields[f.API]
		_, inVer := versionFields[f.API]
		_, inLoc := localizationFields[f.API]
		if !inLoc || (f.Level == metadata.LevelAppInfo) != inInfo || (f.Level == metadata.LevelVersion) != inVer {
			t.Errorf("field %s (%s): info=%v version=%v localization=%v", f.API, f.Level, inInfo, inVer, inLoc)
		}
	}
}

func TestApplyTextChangesPatchesOnlyChanged(t *testing.T) {
	srv := listingServer(t, "app_appInfos.json", "appInfo_localizations_full.json", "version_localizations.json")
	srv.Handle("PATCH", "/v1/appStoreVersionLocalizations/{id}", ascfake.JSON(http.StatusOK, "versionLocalization_created.json"))
	changes := []metadata.Change{{Action: metadata.ActionUpdate, Scope: "version", Locale: "en-US", Field: "keywords", New: "a,b"}}
	applied, err := newService(t, srv).ApplyTextChanges(t.Context(), listingApp, listingVersion, changes)
	if err != nil || len(applied) != 1 {
		t.Fatalf("applied=%v err=%v", applied, err)
	}
	patches := requestsOf(srv, "PATCH")
	if len(patches) != 1 || patches[0].Path != "/v1/appStoreVersionLocalizations/vl-en" {
		t.Fatalf("patches = %v", patches)
	}
	want := `{"data":{"type":"appStoreVersionLocalizations","id":"vl-en","attributes":{"keywords":"a,b"}}}`
	if string(patches[0].Body) != want {
		t.Errorf("body = %s", patches[0].Body)
	}
	if len(requestsOf(srv, "POST")) != 0 {
		t.Error("unexpected POST")
	}
}

func TestApplyTextChangesClearSendsNull(t *testing.T) {
	srv := listingServer(t, "app_appInfos.json", "appInfo_localizations_full.json", "version_localizations.json")
	srv.Handle("PATCH", "/v1/appStoreVersionLocalizations/{id}", ascfake.JSON(http.StatusOK, "versionLocalization_created.json"))
	srv.Handle("PATCH", "/v1/appInfoLocalizations/{id}", ascfake.JSON(http.StatusOK, "appInfoLocalization_created.json"))
	changes := []metadata.Change{
		{Action: metadata.ActionUpdate, Scope: "version", Locale: "en-US", Field: "marketingUrl", Old: "https://example.com"},
		{Action: metadata.ActionUpdate, Scope: "app-info", Locale: "en-US", Field: "subtitle", Old: "Do things"},
	}
	if _, err := newService(t, srv).ApplyTextChanges(t.Context(), listingApp, listingVersion, changes); err != nil {
		t.Fatal(err)
	}
	for _, p := range requestsOf(srv, "PATCH") {
		if !strings.Contains(string(p.Body), `"marketingUrl":null`) && !strings.Contains(string(p.Body), `"subtitle":null`) {
			t.Errorf("body = %s", p.Body)
		}
	}
	if len(requestsOf(srv, "PATCH")) != 2 {
		t.Errorf("patches = %d", len(requestsOf(srv, "PATCH")))
	}
}

func TestApplyTextChangesCreatesLocale(t *testing.T) {
	srv := listingServer(t, "app_appInfos.json", "appInfo_localizations_en_only.json", "version_localizations_en_only.json")
	srv.Handle("POST", "/v1/appInfoLocalizations", ascfake.JSON(http.StatusCreated, "appInfoLocalization_created.json"))
	srv.Handle("POST", "/v1/appStoreVersionLocalizations", ascfake.JSON(http.StatusCreated, "versionLocalization_created.json"))
	changes := []metadata.Change{
		{Action: metadata.ActionCreate, Scope: "app-info", Locale: "fr-FR", Field: "subtitle", New: "Faire"},
		{Action: metadata.ActionCreate, Scope: "version", Locale: "fr-FR", Field: "description", New: "Une app."},
	}
	if _, err := newService(t, srv).ApplyTextChanges(t.Context(), listingApp, listingVersion, changes); err != nil {
		t.Fatal(err)
	}
	posts := requestsOf(srv, "POST")
	if len(posts) != 2 {
		t.Fatalf("posts = %v", posts)
	}
	info, ver := string(posts[0].Body), string(posts[1].Body)
	if !strings.Contains(info, `"locale":"fr-FR"`) || !strings.Contains(info, `"name":"Example"`) || !strings.Contains(info, `"subtitle":"Faire"`) || !strings.Contains(info, `"id":"b7d3f1a0-edit-4c2d-9b4e-000000000002"`) {
		t.Errorf("app info body = %s", info)
	}
	if !strings.Contains(ver, `"locale":"fr-FR"`) || !strings.Contains(ver, `"description":"Une app."`) || !strings.Contains(ver, `"id":"ver-1"`) {
		t.Errorf("version body = %s", ver)
	}
}

func TestApplyTextChangesReturnsAppliedOnError(t *testing.T) {
	srv := listingServer(t, "app_appInfos.json", "appInfo_localizations_full.json", "version_localizations.json")
	srv.Handle("PATCH", "/v1/appStoreVersionLocalizations/vl-en", ascfake.JSON(http.StatusOK, "versionLocalization_created.json"))
	srv.Handle("PATCH", "/v1/appStoreVersionLocalizations/vl-de", ascfake.JSON(http.StatusInternalServerError, "error_409.json"))
	changes := []metadata.Change{
		{Action: metadata.ActionUpdate, Scope: "version", Locale: "en-US", Field: "keywords", New: "a"},
		{Action: metadata.ActionUpdate, Scope: "version", Locale: "de-DE", Field: "keywords", New: "b"},
	}
	applied, err := newService(t, srv).ApplyTextChanges(t.Context(), listingApp, listingVersion, changes)
	if err == nil {
		t.Fatal("no error")
	}
	if len(applied) != 1 || applied[0].Locale != "en-US" {
		t.Errorf("applied = %v", applied)
	}
}

func TestEditableAppInfoLocked(t *testing.T) {
	srv := listingServer(t, "app_appInfos_locked.json", "appInfo_localizations_full.json", "version_localizations.json")
	_, err := newService(t, srv).EditableAppInfo(t.Context(), listingApp)
	var ce *clierr.Error
	if !errors.As(err, &ce) || ce.Message != "app metadata is locked (in review); name, subtitle and privacy URLs cannot change now" {
		t.Fatalf("err = %v", err)
	}
	id, err := newService(t, listingServer(t, "app_appInfos.json", "appInfo_localizations_full.json", "version_localizations.json")).EditableAppInfo(t.Context(), listingApp)
	if err != nil || id != "b7d3f1a0-edit-4c2d-9b4e-000000000002" {
		t.Errorf("id=%q err=%v", id, err)
	}
}

func TestSetListingLockedAppInfo(t *testing.T) {
	srv := listingServer(t, "app_appInfos_locked.json", "appInfo_localizations_full.json", "version_localizations.json")
	_, _, err := newService(t, srv).SetListing(t.Context(), listingApp, listingVersion, "en-US", metadata.Texts{"name": "New"}, false)
	if err == nil || !strings.Contains(err.Error(), "app metadata is locked") {
		t.Fatalf("err = %v", err)
	}
	if len(requestsOf(srv, "PATCH"))+len(requestsOf(srv, "POST")) != 0 {
		t.Error("sent a change")
	}
}

func TestSetListingTooLong(t *testing.T) {
	srv := listingServer(t, "app_appInfos.json", "appInfo_localizations_full.json", "version_localizations.json")
	_, _, err := newService(t, srv).SetListing(t.Context(), listingApp, listingVersion, "en-US", metadata.Texts{"subtitle": strings.Repeat("x", 31)}, false)
	if err == nil || clierr.ExitCode(err) != 1 || !strings.Contains(err.Error(), "en-US/subtitle.txt: 31 characters, limit 30") {
		t.Fatalf("err = %v", err)
	}
}

func TestLocaleRemovePrimaryRefused(t *testing.T) {
	srv := listingServer(t, "app_appInfos.json", "appInfo_localizations_full.json", "version_localizations.json")
	_, err := newService(t, srv).RemoveLocale(t.Context(), listingApp, listingVersion, "en-US", false)
	if err == nil || clierr.ExitCode(err) != 1 || !strings.Contains(err.Error(), "primary locale") {
		t.Fatalf("err = %v", err)
	}
	if len(srv.Requests()) != 0 {
		t.Error("made requests")
	}
}

func TestLocaleAddRemove(t *testing.T) {
	srv := listingServer(t, "app_appInfos.json", "appInfo_localizations_full.json", "version_localizations.json")
	srv.Handle("DELETE", "/v1/appInfoLocalizations/{id}", ascfake.JSON(http.StatusNoContent, "empty.json"))
	srv.Handle("DELETE", "/v1/appStoreVersionLocalizations/{id}", ascfake.JSON(http.StatusNoContent, "empty.json"))
	svc := newService(t, srv)
	res, err := svc.RemoveLocale(t.Context(), listingApp, listingVersion, "de-DE", false)
	if err != nil || res.Action != ActionDelete {
		t.Fatalf("res=%+v err=%v", res, err)
	}
	var paths []string
	for _, r := range requestsOf(srv, "DELETE") {
		paths = append(paths, r.Path)
	}
	slices.Sort(paths)
	if len(paths) != 2 || paths[0] != "/v1/appInfoLocalizations/il-de" || paths[1] != "/v1/appStoreVersionLocalizations/vl-de" {
		t.Errorf("deleted = %v", paths)
	}
	if res, err := svc.AddLocale(t.Context(), listingApp, listingVersion, "de-DE", false); err != nil || res.Action != metadata.ActionSkip {
		t.Errorf("existing locale: res=%+v err=%v", res, err)
	}
}
