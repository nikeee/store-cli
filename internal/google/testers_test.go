package google

import (
	"io"
	"net/http"
	"reflect"
	"slices"
	"testing"

	"github.com/nikeee/store-cli/internal/testutil/playfake"
)

const (
	testersPath = editPath + "/testers/{track}"
	detailsPath = editPath + "/details"
)

func testersServer(t *testing.T) *playfake.Server {
	srv := editServer(t)
	srv.Handle("GET", testersPath, jsonBody(`{"googleGroups":["old@googlegroups.com"]}`))
	srv.Handle("PUT", testersPath, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_, _ = w.Write(b)
	})
	return srv
}

func TestTestersShow(t *testing.T) {
	res, err := newService(t, testersServer(t), io.Discard).ShowTesters(t.Context(), "internal")
	if err != nil {
		t.Fatal(err)
	}
	if res.Track != "internal" || !reflect.DeepEqual(res.GoogleGroups, []string{"old@googlegroups.com"}) {
		t.Errorf("res = %+v", res)
	}
}

func TestTestersSet(t *testing.T) {
	srv := testersServer(t)
	groups := []string{"a@googlegroups.com", "b@example.com"}
	res, err := newService(t, srv, io.Discard).SetTesters(t.Context(), "internal", groups, CommitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	sent := lastBody(t, srv, "PUT")["googleGroups"].([]any)
	if len(sent) != 2 || sent[0] != "a@googlegroups.com" || sent[1] != "b@example.com" {
		t.Errorf("sent = %v", sent)
	}
	if !reflect.DeepEqual(res.GoogleGroups, groups) || res.Track != "internal" {
		t.Errorf("res = %+v", res)
	}
	if c := calls(srv); c[len(c)-1] != "POST /edits/edit-1:commit" {
		t.Errorf("calls = %v", c)
	}
}

func TestTestersSetDryRun(t *testing.T) {
	srv := testersServer(t)
	res, err := newService(t, srv, io.Discard).SetTesters(t.Context(), "internal", []string{"a@b.co"}, CommitOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	noMutations(t, srv)
	if len(res.GoogleGroups) != 1 {
		t.Errorf("res = %+v", res)
	}
	// the dry run reads the track, so a bad track name shows
	if c := calls(srv); !slices.Contains(c, "GET /edits/edit-1/testers/internal") {
		t.Errorf("calls = %v", c)
	}
	srv.Handle("GET", testersPath, func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusNotFound)
		_, _ = io.WriteString(w, `{"error":{"code":404,"message":"Track not found.","status":"NOT_FOUND"}}`)
	})
	if _, err := newService(t, srv, io.Discard).SetTesters(t.Context(), "nope", []string{"a@b.co"}, CommitOptions{DryRun: true}); err == nil {
		t.Error("unknown track accepted")
	}
}

func detailsServer(t *testing.T) *playfake.Server {
	srv := editServer(t)
	srv.Handle("GET", detailsPath, jsonBody(`{"contactEmail":"old@example.com","contactWebsite":"https://old.example","contactPhone":"+1","defaultLanguage":"cs-CZ"}`))
	srv.Handle("PATCH", detailsPath, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_, _ = w.Write(b)
	})
	return srv
}

func ptr(s string) *string { return &s }

func TestDetailsShowMapsLocale(t *testing.T) {
	res, err := newService(t, detailsServer(t), io.Discard).ShowDetails(t.Context())
	if err != nil {
		t.Fatal(err)
	}
	want := Details{Email: "old@example.com", Website: "https://old.example", Phone: "+1", DefaultLocale: "cs"}
	if res != want {
		t.Errorf("res = %+v", res)
	}
}

func TestDetailsPatchOnlyGiven(t *testing.T) {
	srv := detailsServer(t)
	_, err := newService(t, srv, io.Discard).SetDetails(t.Context(), DetailsPatch{Email: ptr("new@example.com"), Phone: ptr("")}, CommitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	body := lastBody(t, srv, "PATCH")
	if len(body) != 2 || body["contactEmail"] != "new@example.com" || body["contactPhone"] != "" {
		t.Errorf("body = %v", body)
	}
}

func TestDetailsDefaultLocaleMapped(t *testing.T) {
	srv := detailsServer(t)
	res, err := newService(t, srv, io.Discard).SetDetails(t.Context(), DetailsPatch{DefaultLocale: ptr("zh-Hans")}, CommitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	body := lastBody(t, srv, "PATCH")
	if len(body) != 1 || body["defaultLanguage"] != "zh-CN" || res.DefaultLocale != "zh-Hans" {
		t.Errorf("body = %v res = %+v", body, res)
	}
}

func TestDetailsSetDryRun(t *testing.T) {
	srv := detailsServer(t)
	res, err := newService(t, srv, io.Discard).SetDetails(t.Context(), DetailsPatch{Website: ptr("https://new.example")}, CommitOptions{DryRun: true})
	if err != nil {
		t.Fatal(err)
	}
	noMutations(t, srv)
	if res.Website != "https://new.example" || res.Email != "old@example.com" {
		t.Errorf("res = %+v", res)
	}
}
