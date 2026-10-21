package cli_test

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/testutil/golden"
	"github.com/nikeee/store-cli/internal/testutil/playfake"
)

const (
	editListingsPath = googleEditsPath + "/edit-1/listings"
	editListingPath  = editListingsPath + "/{language}"
)

// playListingServer serves it-IT (with video, multi-line full description) and
// en-US; PUTs are echoed and other languages are a 404.
func playListingServer(t *testing.T) *playfake.Server {
	srv := googleEditServer(t)
	const itIT = `{"language":"it-IT","title":"Titolo","shortDescription":"Breve","fullDescription":"Riga uno\nRiga due","video":"https://youtu.be/x"}`
	const enUS = `{"language":"en-US","title":"Title","shortDescription":"Short"}`
	srv.Handle("GET", editListingsPath, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"listings":[`+itIT+`,`+enUS+`]}`)
	})
	srv.Handle("GET", editListingPath, func(w http.ResponseWriter, r *http.Request) {
		switch r.PathValue("language") {
		case "it-IT":
			_, _ = io.WriteString(w, itIT)
		case "en-US":
			_, _ = io.WriteString(w, enUS)
		default:
			w.WriteHeader(http.StatusNotFound)
			_, _ = io.WriteString(w, `{"error":{"code":404,"message":"not found","status":"NOT_FOUND"}}`)
		}
	})
	srv.Handle("PUT", editListingPath, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_, _ = w.Write(b)
	})
	return srv
}

func putBody(srv *playfake.Server) (path, body string) {
	for _, r := range srv.Requests() {
		if r.Method == "PUT" {
			path, body = r.Path, string(r.Body)
		}
	}
	return path, body
}

func TestGoogleListingShowGolden(t *testing.T) {
	for _, mode := range []string{"text", "json", "toon"} {
		t.Run(mode, func(t *testing.T) {
			srv := playListingServer(t)
			code, stdout, stderr := runEnv(t, googleEnv(srv, nil), false, "google", "listing", "show", "--app", "com.example.app", "-o", mode)
			if code != 0 || stderr != "" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			golden.Assert(t, "google_listing_show."+mode, []byte(stdout))
		})
	}
}

func TestGoogleListingSetGolden(t *testing.T) {
	for _, mode := range []string{"text", "json", "toon"} {
		t.Run(mode, func(t *testing.T) {
			srv := playListingServer(t)
			code, stdout, stderr := runEnv(t, googleEnv(srv, nil), false, "google", "listing", "set", "--app", "com.example.app", "--locale", "en-US", "--title", "New title", "-o", mode)
			if code != 0 || stderr != "" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			golden.Assert(t, "google_listing_set."+mode, []byte(stdout))
		})
	}
}

func TestListingSetKeepsUntouchedFields(t *testing.T) {
	srv := playListingServer(t)
	code, _, stderr := runEnv(t, googleEnv(srv, nil), false, "google", "listing", "set", "--app", "com.example.app", "--locale", "it", "--title", "Nuovo")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	_, body := putBody(srv)
	if !strings.Contains(body, `"video":"https://youtu.be/x"`) || !strings.Contains(body, `"title":"Nuovo"`) || !strings.Contains(body, `"shortDescription":"Breve"`) {
		t.Errorf("body = %s", body)
	}
}

func TestListingSetNewLocale(t *testing.T) {
	srv := playListingServer(t)
	code, _, stderr := runEnv(t, googleEnv(srv, nil), false, "google", "listing", "set", "--app", "com.example.app", "--locale", "fr-FR", "--title", "Titre")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	path, body := putBody(srv)
	if !strings.HasSuffix(path, "/listings/fr-FR") || strings.TrimSpace(body) != `{"language":"fr-FR","title":"Titre"}` {
		t.Errorf("path=%s body=%s", path, body)
	}
}

func TestListingSetMapsLocale(t *testing.T) {
	srv := playListingServer(t)
	code, _, stderr := runEnv(t, googleEnv(srv, nil), false, "google", "listing", "set", "--app", "com.example.app", "--locale", "it", "--title", "Nuovo")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if path, _ := putBody(srv); !strings.HasSuffix(path, "/listings/it-IT") {
		t.Errorf("path = %s", path)
	}
}

func TestListingSetValidation(t *testing.T) {
	srv := playListingServer(t)
	code, _, stderr := runEnv(t, googleEnv(srv, nil), false, "google", "listing", "set", "--app", "com.example.app", "--locale", "en-US", "--title", strings.Repeat("x", 31))
	if code != 1 || !strings.Contains(stderr, "en-US/title.txt: 31 characters, limit 30") || len(srv.Requests()) != 0 {
		t.Errorf("code=%d stderr=%q requests=%d", code, stderr, len(srv.Requests()))
	}
}

func TestListingUsageErrorsSendNothing(t *testing.T) {
	app := []string{"--app", "com.example.app"}
	for name, args := range map[string][]string{
		"set no locale":  {"google", "listing", "set", "--title", "x"},
		"set bad locale": {"google", "listing", "set", "--locale", "english!", "--title", "x"},
		"set no field":   {"google", "listing", "set", "--locale", "en-US"},
		"show bad":       {"google", "listing", "show", "--locale", "english!"},
	} {
		srv := playListingServer(t)
		code, _, stderr := runEnv(t, googleEnv(srv, nil), false, append(args, app...)...)
		if code != 2 || len(srv.Requests()) != 0 {
			t.Errorf("%s: code=%d stderr=%q requests=%d", name, code, stderr, len(srv.Requests()))
		}
	}
}

func TestListingSetDryRun(t *testing.T) {
	srv := playListingServer(t)
	code, stdout, stderr := runEnv(t, googleEnv(srv, nil), false, "google", "listing", "set", "--app", "com.example.app", "--locale", "en-US", "--title", "New", "--dry-run", "-o", "json")
	if code != 0 || !strings.Contains(stdout, `"title": "New"`) {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	for _, r := range srv.Requests() {
		if r.Method == "PUT" || strings.HasSuffix(r.Path, ":commit") {
			t.Errorf("dry run sent %s %s", r.Method, r.Path)
		}
	}
	if methods(srv, "DELETE") != 1 {
		t.Errorf("edit not deleted")
	}
}

func TestListingSetChangesNotSentForReview(t *testing.T) {
	srv := playListingServer(t)
	code, _, stderr := runEnv(t, googleEnv(srv, nil), false, "google", "listing", "set", "--app", "com.example.app", "--locale", "en-US", "--title", "New", "--changes-not-sent-for-review")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	found := false
	for _, r := range srv.Requests() {
		found = found || (strings.HasSuffix(r.Path, ":commit") && r.Query.Get("changesNotSentForReview") == "true")
	}
	if !found {
		t.Error("commit without changesNotSentForReview")
	}
}
