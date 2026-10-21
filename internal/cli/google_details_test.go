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
	editTestersPath = googleEditsPath + "/edit-1/testers/{track}"
	editDetailsPath = googleEditsPath + "/edit-1/details"
)

func detailsCLIServer(t *testing.T) *playfake.Server {
	srv := googleEditServer(t)
	srv.Handle("GET", editTestersPath, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"googleGroups":["qa@googlegroups.com","beta@example.com"]}`)
	})
	srv.Handle("PUT", editTestersPath, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_, _ = w.Write(b)
	})
	srv.Handle("GET", editDetailsPath, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"contactEmail":"support@example.com","contactWebsite":"https://example.com","contactPhone":"+49 30 1234","defaultLanguage":"en-US"}`)
	})
	srv.Handle("PATCH", editDetailsPath, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_, _ = w.Write(b)
	})
	return srv
}

func TestGoogleTestersDetailsGolden(t *testing.T) {
	for _, mode := range []string{"text", "json", "toon"} {
		t.Run(mode, func(t *testing.T) {
			for name, args := range map[string][]string{
				"google_track_testers_show": {"google", "track", "testers", "show", "internal"},
				"google_details_show":       {"google", "details", "show"},
			} {
				srv := detailsCLIServer(t)
				code, stdout, stderr := runEnv(t, googleEnv(srv, nil), false, append(args, "--app", "com.example.app", "-o", mode)...)
				if code != 0 || stderr != "" {
					t.Fatalf("%s: code=%d stderr=%q", name, code, stderr)
				}
				golden.Assert(t, name+"."+mode, []byte(stdout))
			}
		})
	}
}

func TestGoogleTestersSet(t *testing.T) {
	srv := detailsCLIServer(t)
	code, stdout, stderr := runEnv(t, googleEnv(srv, nil), false,
		"google", "track", "testers", "set", "internal", "--app", "com.example.app", "--group", "a@googlegroups.com", "--group", "b@example.com", "-o", "json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	var put string
	for _, r := range srv.Requests() {
		if r.Method == "PUT" {
			put = string(r.Body)
		}
	}
	if !strings.Contains(put, "a@googlegroups.com") || !strings.Contains(put, "b@example.com") || !strings.Contains(stdout, "b@example.com") {
		t.Errorf("put=%s stdout=%s", put, stdout)
	}
}

func TestGoogleDetailsSetMapsLocale(t *testing.T) {
	srv := detailsCLIServer(t)
	code, _, stderr := runEnv(t, googleEnv(srv, nil), false,
		"google", "details", "set", "--app", "com.example.app", "--default-locale", "ja", "--email", "x@example.com")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	var patch string
	for _, r := range srv.Requests() {
		if r.Method == "PATCH" {
			patch = string(r.Body)
		}
	}
	if !strings.Contains(patch, `"defaultLanguage":"ja-JP"`) || !strings.Contains(patch, `"contactEmail":"x@example.com"`) || strings.Contains(patch, "contactPhone") {
		t.Errorf("patch=%s", patch)
	}
}

func TestGoogleTestersDetailsUsageErrorsSendNothing(t *testing.T) {
	app := []string{"--app", "com.example.app"}
	for name, args := range map[string][]string{
		"testers no group":     {"google", "track", "testers", "set", "internal"},
		"testers bad group":    {"google", "track", "testers", "set", "internal", "--group", "not-an-email"},
		"testers one bad":      {"google", "track", "testers", "set", "internal", "--group", "a@b.co", "--group", "x"},
		"details no field":     {"google", "details", "set"},
		"details bad locale":   {"google", "details", "set", "--default-locale", "english"},
		"details empty locale": {"google", "details", "set", "--default-locale", ""},
	} {
		srv := detailsCLIServer(t)
		code, _, stderr := runEnv(t, googleEnv(srv, nil), false, append(args, app...)...)
		if code != 2 || len(srv.Requests()) != 0 {
			t.Errorf("%s: code=%d stderr=%q requests=%d", name, code, stderr, len(srv.Requests()))
		}
	}
}

func TestGoogleTestersDetailsDryRun(t *testing.T) {
	for _, args := range [][]string{
		{"track", "testers", "set", "internal", "--group", "a@b.co"},
		{"details", "set", "--email", "x@example.com"},
		{"details", "set", "--default-locale", "de-DE"},
	} {
		srv := detailsCLIServer(t)
		code, stdout, stderr := runEnv(t, googleEnv(srv, nil), false, append(append([]string{"google"}, args...), "--app", "com.example.app", "--dry-run", "-o", "json")...)
		if code != 0 || stdout == "" {
			t.Fatalf("%v: code=%d stdout=%q stderr=%q", args, code, stdout, stderr)
		}
		for _, r := range srv.Requests() {
			if r.Method == "PUT" || r.Method == "PATCH" || strings.HasSuffix(r.Path, ":commit") {
				t.Errorf("%v: dry run sent %s %s", args, r.Method, r.Path)
			}
		}
		if methods(srv, "DELETE") != 1 {
			t.Errorf("%v: edit not deleted", args)
		}
	}
}

func TestGoogleTestersDetailsChangesNotSentForReview(t *testing.T) {
	for _, args := range [][]string{
		{"track", "testers", "set", "internal", "--group", "a@b.co"},
		{"details", "set", "--phone", "123"},
	} {
		srv := detailsCLIServer(t)
		code, _, stderr := runEnv(t, googleEnv(srv, nil), false, append(append([]string{"google"}, args...), "--app", "com.example.app", "--changes-not-sent-for-review")...)
		if code != 0 {
			t.Fatalf("%v: code=%d stderr=%q", args, code, stderr)
		}
		found := false
		for _, r := range srv.Requests() {
			found = found || strings.HasSuffix(r.Path, ":commit") && r.Query.Get("changesNotSentForReview") == "true"
		}
		if !found {
			t.Errorf("%v: commit without changesNotSentForReview=true", args)
		}
	}
}
