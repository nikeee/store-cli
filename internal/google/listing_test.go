package google

import (
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/metadata"
	"github.com/nikeee/store-cli/internal/testutil/playfake"
)

const listingPath = listingsPath + "/{language}"

// listingServer serves en-US (with video) and de-DE; PUTs are echoed and an
// unknown language is a 404.
func listingServer(t *testing.T) *playfake.Server {
	srv := editServer(t)
	srv.Handle("GET", listingsPath, jsonBody(`{"listings":[
		{"language":"en-US","title":"Title","shortDescription":"Short","fullDescription":"Full","video":"https://youtu.be/x"},
		{"language":"de-DE","title":"Titel"}]}`))
	srv.Handle("GET", listingPath, func(w http.ResponseWriter, r *http.Request) {
		switch r.PathValue("language") {
		case "en-US":
			jsonBody(`{"language":"en-US","title":"Title","shortDescription":"Short","fullDescription":"Full","video":"https://youtu.be/x"}`)(w, r)
		case "de-DE":
			jsonBody(`{"language":"de-DE","title":"Titel"}`)(w, r)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"error":{"code":404,"message":"not found","status":"NOT_FOUND"}}`)
		}
	})
	srv.Handle("PUT", listingPath, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_, _ = w.Write(b)
	})
	return srv
}

func TestListingTexts(t *testing.T) {
	srv := listingServer(t)
	var got map[string]metadata.Texts
	s := newService(t, srv, io.Discard)
	err := s.withEdit(t.Context(), readOnly, func(editID string) error {
		var err error
		got, err = s.ListingTexts(t.Context(), editID)
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]metadata.Texts{
		"en-US": {"title": "Title", "shortDescription": "Short", "fullDescription": "Full", "video": "https://youtu.be/x"},
		"de-DE": {"title": "Titel"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v", got)
	}
}

func TestSetListingKeepsUntouchedFields(t *testing.T) {
	srv := listingServer(t)
	res, err := newService(t, srv, io.Discard).SetListing(t.Context(), "en-US", metadata.Texts{"title": "New"}, CommitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	put := lastBody(t, srv, "PUT")
	if put["title"] != "New" || put["video"] != "https://youtu.be/x" || put["fullDescription"] != "Full" || put["shortDescription"] != "Short" {
		t.Errorf("put = %v", put)
	}
	if res != (Listing{Locale: "en-US", Title: "New", ShortDescription: "Short", FullDescription: "Full", Video: "https://youtu.be/x"}) {
		t.Errorf("res = %+v", res)
	}
	if c := calls(srv); c[len(c)-1] != "POST /edits/edit-1:commit" {
		t.Errorf("calls = %v", c)
	}
}

func TestSetListingEmptyClearsField(t *testing.T) {
	srv := listingServer(t)
	res, err := newService(t, srv, io.Discard).SetListing(t.Context(), "en-US", metadata.Texts{"video": ""}, CommitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	put := lastBody(t, srv, "PUT")
	if _, ok := put["video"]; ok || put["title"] != "Title" || res.Video != "" {
		t.Errorf("put = %v res = %+v", put, res)
	}
}

func TestSetListingNewLocale(t *testing.T) {
	srv := listingServer(t)
	res, err := newService(t, srv, io.Discard).SetListing(t.Context(), "fr-FR", metadata.Texts{"title": "Titre"}, CommitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	put := lastBody(t, srv, "PUT")
	if put["language"] != "fr-FR" || put["title"] != "Titre" || len(put) != 2 {
		t.Errorf("put = %v", put)
	}
	if res.Locale != "fr-FR" || res.Title != "Titre" {
		t.Errorf("res = %+v", res)
	}
}

func TestSetListingValidationSendsNothing(t *testing.T) {
	srv := listingServer(t)
	_, err := newService(t, srv, io.Discard).SetListing(t.Context(), "en-US", metadata.Texts{"title": strings.Repeat("x", 31)}, CommitOptions{})
	if err == nil || clierr.ExitCode(err) != 1 || !strings.Contains(err.Error(), "en-US/title.txt: 31 characters, limit 30") {
		t.Fatalf("err = %v", err)
	}
	if n := len(srv.Requests()); n != 0 {
		t.Errorf("%d requests sent", n)
	}
}

func TestSetListingNoChangeDoesNotPut(t *testing.T) {
	srv := listingServer(t)
	res, err := newService(t, srv, io.Discard).SetListing(t.Context(), "en-US", metadata.Texts{"title": "Title"}, CommitOptions{})
	if err != nil || res.Title != "Title" {
		t.Fatalf("res = %+v err = %v", res, err)
	}
	for _, r := range srv.Requests() {
		if r.Method == "PUT" || strings.HasSuffix(r.Path, ":commit") {
			t.Errorf("sent %s %s", r.Method, r.Path)
		}
	}
}

func TestSetListingDryRun(t *testing.T) {
	srv := listingServer(t)
	res, err := newService(t, srv, io.Discard).SetListing(t.Context(), "en-US", metadata.Texts{"title": "New"}, CommitOptions{DryRun: true})
	if err != nil || res.Title != "New" || res.Video != "https://youtu.be/x" {
		t.Fatalf("res = %+v err = %v", res, err)
	}
	noMutations(t, srv)
	if c := calls(srv); c[len(c)-1] != "DELETE /edits/edit-1" {
		t.Errorf("calls = %v", c)
	}
}

func TestShowListings(t *testing.T) {
	srv := listingServer(t)
	s := newService(t, srv, io.Discard)
	all, err := s.ShowListings(t.Context(), "")
	if err != nil || len(all) != 2 || all[0].Locale != "de-DE" || all[1].Video != "https://youtu.be/x" {
		t.Fatalf("all = %+v err = %v", all, err)
	}
	one, err := s.ShowListings(t.Context(), "de-DE")
	if err != nil || len(one) != 1 || one[0].Title != "Titel" {
		t.Fatalf("one = %+v err = %v", one, err)
	}
	if _, err := s.ShowListings(t.Context(), "fr-FR"); err == nil || !strings.Contains(err.Error(), "fr-FR") {
		t.Errorf("err = %v", err)
	}
}

func TestApplyListingChangesReturnsAppliedOnError(t *testing.T) {
	srv := listingServer(t)
	n := 0
	srv.Handle("PUT", listingPath, func(w http.ResponseWriter, r *http.Request) {
		n++
		if n == 2 {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		b, _ := io.ReadAll(r.Body)
		_, _ = w.Write(b)
	})
	changes := []metadata.Change{
		{Action: metadata.ActionUpdate, Scope: "listing", Locale: "en-US", Field: "title", New: "A"},
		{Action: metadata.ActionUpdate, Scope: "listing", Locale: "en-US", Field: "video", New: "v"},
		{Action: metadata.ActionUpdate, Scope: "listing", Locale: "de-DE", Field: "title", New: "B"},
	}
	var applied []metadata.Change
	s := newService(t, srv, io.Discard)
	_ = s.withEdit(t.Context(), CommitOptions{DryRun: true}, func(editID string) error {
		var err error
		applied, err = s.ApplyListingChanges(t.Context(), editID, changes)
		if err == nil {
			t.Error("want error")
		}
		return nil
	})
	if len(applied) != 2 {
		t.Errorf("applied = %v", applied)
	}
}
