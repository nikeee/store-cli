package cli_test

import (
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/testutil/golden"
	"github.com/nikeee/store-cli/internal/testutil/playfake"
)

const (
	internalTrackJSON = `{"track":"internal","releases":[
		{"name":"v1.3.0","status":"completed","versionCodes":["30"]},
		{"name":"v1.4.0","status":"completed","versionCodes":["40","41"],"releaseNotes":[{"language":"en-US","text":"Bug fixes"},{"language":"de-DE","text":"Fehler behoben"}]}]}`
	stagedTrackJSON = `{"track":"staged","releases":[
		{"name":"v1.3.0","status":"completed","versionCodes":["30"]},
		{"name":"v1.4.0","status":"inProgress","userFraction":0.2,"versionCodes":["40"],"releaseNotes":[{"language":"en-US","text":"Bug fixes"}]}]}`
	haltedTrackJSON = `{"track":"stopped","releases":[
		{"name":"v1.4.0","status":"halted","userFraction":0.2,"versionCodes":["40"]}]}`
)

func trackCLIServer(t *testing.T) *playfake.Server {
	srv := googleEditServer(t)
	tracks := map[string]string{"internal": internalTrackJSON, "staged": stagedTrackJSON, "stopped": haltedTrackJSON}
	srv.Handle("GET", googleEditsPath+"/edit-1/listings", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"listings":[{"language":"en-US"},{"language":"de-DE"}]}`)
	})
	srv.Handle("GET", googleEditsPath+"/edit-1/tracks", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"tracks":[`+internalTrackJSON+`,`+stagedTrackJSON+`]}`)
	})
	srv.Handle("GET", editTrackPath, func(w http.ResponseWriter, r *http.Request) {
		body, ok := tracks[r.PathValue("track")]
		if !ok {
			body = `{"track":"` + r.PathValue("track") + `"}`
		}
		_, _ = io.WriteString(w, body)
	})
	srv.Handle("PUT", editTrackPath, func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		_, _ = w.Write(b)
	})
	return srv
}

func trackArgs(args ...string) []string {
	return append([]string{"google", "track", "--app", "com.example.app"}, args...)
}

func TestGoogleTrackListShowGolden(t *testing.T) {
	for _, mode := range []string{"text", "json", "toon"} {
		t.Run(mode, func(t *testing.T) {
			srv := trackCLIServer(t)
			code, stdout, stderr := runEnv(t, googleEnv(srv, nil), false, append(trackArgs("list"), "-o", mode)...)
			if code != 0 || stderr != "" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			golden.Assert(t, "google_track_list."+mode, []byte(stdout))
			code, stdout, stderr = runEnv(t, googleEnv(srv, nil), false, append(trackArgs("show", "staged"), "-o", mode)...)
			if code != 0 || stderr != "" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			golden.Assert(t, "google_track_show."+mode, []byte(stdout))
		})
	}
}

func TestGoogleTrackPromote(t *testing.T) {
	srv := trackCLIServer(t)
	code, stdout, stderr := runEnv(t, googleEnv(srv, nil), false, append(trackArgs("promote", "--from", "internal", "--to", "production", "--rollout", "0.1"), "-o", "json")...)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	var put string
	for _, r := range srv.Requests() {
		if r.Method == "PUT" {
			put = string(r.Body)
		}
	}
	for _, want := range []string{`"track":"production"`, `"name":"v1.4.0"`, `"status":"inProgress"`, `"userFraction":0.1`, `"40","41"`, `Fehler behoben`} {
		if !strings.Contains(put, want) {
			t.Errorf("body lacks %s: %s", want, put)
		}
	}
	if !strings.Contains(stdout, `"track": "production"`) && !strings.Contains(stdout, `"track":"production"`) {
		t.Errorf("stdout=%q", stdout)
	}
}

func TestGoogleTrackLocalValidationSendsNothing(t *testing.T) {
	long := t.TempDir()
	_ = os.WriteFile(filepath.Join(long, "en-US.txt"), []byte(strings.Repeat("x", 501)), 0o600)
	for name, tt := range map[string]struct {
		args []string
		code int
	}{
		"promote missing from":         {trackArgs("promote", "--to", "production"), 2},
		"promote in-progress no ratio": {trackArgs("promote", "--from", "a", "--to", "b", "--status", "in-progress"), 2},
		"promote halted no ratio":      {trackArgs("promote", "--from", "a", "--to", "b", "--status", "halted"), 2},
		"promote bad rollout":          {trackArgs("promote", "--from", "a", "--to", "b", "--rollout", "1"), 2},
		"promote bad status":           {trackArgs("promote", "--from", "a", "--to", "b", "--status", "x"), 2},
		"promote notes dir missing":    {trackArgs("promote", "--from", "a", "--to", "b", "--release-notes", filepath.Join(long, "none")), 1},
		"promote notes too long":       {trackArgs("promote", "--from", "a", "--to", "b", "--release-notes", long), 1},
		"rollout zero":                 {trackArgs("rollout", "production", "0"), 2},
		"rollout one":                  {trackArgs("rollout", "production", "1"), 2},
		"rollout text":                 {trackArgs("rollout", "production", "half"), 2},
		"notes no input":               {[]string{"google", "release-notes", "set", "production", "--app", "com.example.app"}, 2},
		"notes dir and text":           {[]string{"google", "release-notes", "set", "production", "--app", "com.example.app", "--dir", long, "--text", "x"}, 2},
		"notes too long":               {[]string{"google", "release-notes", "set", "production", "--app", "com.example.app", "--dir", long}, 1},
		"notes text too long":          {[]string{"google", "release-notes", "set", "production", "--app", "com.example.app", "--locale", "en-US", "--text", strings.Repeat("ä", 501)}, 1},
		"notes dir missing":            {[]string{"google", "release-notes", "set", "production", "--app", "com.example.app", "--dir", filepath.Join(long, "none")}, 1},
	} {
		srv := trackCLIServer(t)
		code, _, stderr := runEnv(t, googleEnv(srv, nil), false, tt.args...)
		if code != tt.code || len(srv.Requests()) != 0 {
			t.Errorf("%s: code=%d want %d stderr=%q requests=%d", name, code, tt.code, stderr, len(srv.Requests()))
		}
	}
}

func TestBundleUploadStagedStatusNeedsRollout(t *testing.T) {
	for _, status := range []string{"in-progress", "halted"} {
		srv := bundleCLIServer(t)
		code, _, stderr := runEnv(t, googleEnv(srv, nil), false, bundleArgs(aabFile(t), "--track", "x", "--status", status)...)
		if code != 2 || !strings.Contains(stderr, "--rollout") || len(srv.Requests()) != 0 {
			t.Errorf("%s: code=%d stderr=%q requests=%d", status, code, stderr, len(srv.Requests()))
		}
	}
	srv := bundleCLIServer(t)
	code, _, stderr := runEnv(t, googleEnv(srv, nil), false, bundleArgs(aabFile(t), "--track", "x", "--status", "halted", "--rollout", "0.3")...)
	if code != 0 {
		t.Errorf("halted with rollout: code=%d stderr=%q", code, stderr)
	}
}

func TestGoogleTrackMutationsDryRun(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "en-US.txt"), []byte("Fixes\n"), 0o600)
	for _, args := range [][]string{
		{"track", "promote", "--from", "internal", "--to", "production"},
		{"track", "promote", "--from", "internal", "--to", "production", "--rollout", "0.1"},
		{"track", "rollout", "staged", "0.5"},
		{"track", "halt", "staged"},
		{"track", "resume", "stopped"},
		{"track", "complete", "staged"},
		{"release-notes", "set", "internal", "--locale", "en-US", "--text", "x"},
		{"release-notes", "set", "internal", "--dir", dir},
	} {
		srv := trackCLIServer(t)
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

func TestGoogleTrackChangesNotSentForReview(t *testing.T) {
	srv := trackCLIServer(t)
	code, _, stderr := runEnv(t, googleEnv(srv, nil), false, trackArgs("halt", "staged", "--changes-not-sent-for-review")...)
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	found := false
	for _, r := range srv.Requests() {
		found = found || strings.HasSuffix(r.Path, ":commit") && r.Query.Get("changesNotSentForReview") == "true"
	}
	if !found {
		t.Error("commit without changesNotSentForReview=true")
	}
}

func TestGoogleReleaseNotesSet(t *testing.T) {
	srv := trackCLIServer(t)
	code, stdout, stderr := runEnv(t, googleEnv(srv, nil), false,
		"google", "release-notes", "set", "internal", "--app", "com.example.app", "--release-name", "v1.3.0", "--locale", "en-US", "--text", "Hello")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, "release: v1.3.0") || !strings.Contains(stdout, "en-US: Hello") {
		t.Errorf("stdout=%q", stdout)
	}
}
