package apple

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/testutil/ascfake"
)

var testBuild = Build{ID: "b3f1c2d4-0000-4a11-9c01-000000000044", Build: "44", Version: "1.3.0", Platform: "ios"}

func notesServer(t *testing.T, list string) *ascfake.Server {
	t.Helper()
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/builds/{id}/betaBuildLocalizations", ascfake.JSON(http.StatusOK, list))
	srv.Handle("POST", "/v1/betaBuildLocalizations", ascfake.JSON(http.StatusCreated, "beta_build_localization.json"))
	srv.Handle("PATCH", "/v1/betaBuildLocalizations/{id}", ascfake.JSON(http.StatusOK, "beta_build_localization.json"))
	return srv
}

func TestSetBuildNotesCreates(t *testing.T) {
	srv := notesServer(t, "beta_build_localizations_empty.json")
	got, err := newService(t, srv).SetBuildNotes(t.Context(), testBuild, "en-US", "Fixed login", false)
	if err != nil {
		t.Fatal(err)
	}
	if got != (BuildNotes{Build: "44", Locale: "en-US", Text: "Fixed login"}) {
		t.Errorf("notes = %+v", got)
	}
	reqs := srv.Requests()
	if len(reqs) != 2 || reqs[1].Method != "POST" || reqs[1].Path != "/v1/betaBuildLocalizations" {
		t.Fatalf("requests = %+v", reqs)
	}
	var body map[string]any
	if err := json.Unmarshal(reqs[1].Body, &body); err != nil {
		t.Fatal(err)
	}
	want := `{"data":{"type":"betaBuildLocalizations","attributes":{"whatsNew":"Fixed login","locale":"en-US"},"relationships":{"build":{"data":{"type":"builds","id":"` + testBuild.ID + `"}}}}}`
	wantBody := map[string]any{}
	_ = json.Unmarshal([]byte(want), &wantBody)
	if a, b := mustJSON(body), mustJSON(wantBody); a != b {
		t.Errorf("body = %s, want %s", a, b)
	}
}

func TestSetBuildNotesUpdates(t *testing.T) {
	srv := notesServer(t, "beta_build_localizations_one.json")
	got, err := newService(t, srv).SetBuildNotes(t.Context(), testBuild, "en-US", "Fixed login", false)
	if err != nil {
		t.Fatal(err)
	}
	if got.Text != "Fixed login" || got.Locale != "en-US" {
		t.Errorf("notes = %+v", got)
	}
	reqs := srv.Requests()
	if len(reqs) != 2 || reqs[1].Method != "PATCH" || reqs[1].Path != "/v1/betaBuildLocalizations/loc-en" {
		t.Fatalf("requests = %+v", reqs)
	}
	want := `{"data":{"type":"betaBuildLocalizations","id":"loc-en","attributes":{"whatsNew":"Fixed login"}}}`
	if string(reqs[1].Body) != want {
		t.Errorf("body = %s, want %s", reqs[1].Body, want)
	}
}

func TestSetBuildNotesTooLong(t *testing.T) {
	srv := ascfake.New(t)
	svc := newService(t, srv)
	// 4000 runes of a multi-byte character are fine; 4001 are not.
	if err := CheckBuildNotes(strings.Repeat("é", 4000)); err != nil {
		t.Fatalf("4000 characters: %v", err)
	}
	if _, err := svc.SetBuildNotes(t.Context(), testBuild, "en-US", strings.Repeat("é", 4001), true); err == nil {
		t.Fatal("want error for 4001 characters")
	}
	_, err := svc.SetBuildNotes(t.Context(), testBuild, "en-US", strings.Repeat("é", 4001), false)
	if err == nil || !strings.Contains(err.Error(), "4000") {
		t.Fatalf("err = %v", err)
	}
	if n := len(srv.Requests()); n != 0 {
		t.Errorf("sent %d requests before validating", n)
	}
}

func mustJSON(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}
