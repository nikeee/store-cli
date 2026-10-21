package google

import (
	"io"
	"net/http"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/testutil/playfake"
)

// tracksServer serves the given track bodies (by track name) and an empty
// track for the others; listings are en-US and de-DE, track updates are echoed.
func tracksServer(t *testing.T, tracks map[string]string) *playfake.Server {
	srv := bundleServer(t, `{}`)
	srv.Handle("GET", trackPath, func(w http.ResponseWriter, r *http.Request) {
		name := r.PathValue("track")
		body, ok := tracks[name]
		if !ok {
			body = `{"track":"` + name + `"}`
		}
		jsonBody(body)(w, r)
	})
	return srv
}

func sentReleases(t *testing.T, srv *playfake.Server) []map[string]any {
	t.Helper()
	var out []map[string]any
	for _, r := range lastBody(t, srv, "PUT")["releases"].([]any) {
		out = append(out, r.(map[string]any))
	}
	return out
}

func noMutations(t *testing.T, srv *playfake.Server) {
	t.Helper()
	for _, r := range srv.Requests() {
		if r.Method == "PUT" || r.Method == "PATCH" || strings.HasSuffix(r.Path, ":commit") {
			t.Errorf("mutating request %s %s", r.Method, r.Path)
		}
	}
}

const internalTrack = `{"track":"internal","releases":[
	{"name":"v1","status":"completed","versionCodes":["10"]},
	{"name":"v3","status":"completed","versionCodes":["30","31"],"releaseNotes":[{"language":"en-US","text":"Three"}]},
	{"name":"v2","status":"completed","versionCodes":["20"]}]}`

func TestPromote(t *testing.T) {
	srv := tracksServer(t, map[string]string{
		"internal":   internalTrack,
		"production": `{"track":"production","releases":[{"name":"old","status":"completed","versionCodes":["5"]}]}`,
	})
	res, err := newService(t, srv, io.Discard).Promote(t.Context(), PromoteOptions{
		Name: "v2", From: "internal", To: "production", Status: StatusCompleted,
	})
	if err != nil {
		t.Fatal(err)
	}
	body := lastBody(t, srv, "PUT")
	rel := sentReleases(t, srv)
	if body["track"] != "production" || len(rel) != 1 || rel[0]["name"] != "v2" || rel[0]["status"] != "completed" ||
		rel[0]["versionCodes"].([]any)[0] != "20" || rel[0]["userFraction"] != nil {
		t.Errorf("body = %v", body)
	}
	if c := strings.Join(calls(srv), ","); !strings.HasSuffix(c, "PUT /edits/edit-1/tracks/production,POST /edits/edit-1:commit") {
		t.Errorf("calls = %s", c)
	}
	if res.Track != "production" || res.Releases[0].Name != "v2" {
		t.Errorf("res = %+v", res)
	}
}

func TestPromoteNewestByVersionCode(t *testing.T) {
	srv := tracksServer(t, map[string]string{"internal": internalTrack})
	s := newService(t, srv, io.Discard)
	if _, err := s.Promote(t.Context(), PromoteOptions{From: "internal", To: "beta", Status: StatusCompleted}); err != nil {
		t.Fatal(err)
	}
	rel := sentReleases(t, srv)[0]
	if rel["name"] != "v3" || len(rel["versionCodes"].([]any)) != 2 {
		t.Errorf("release = %v", rel)
	}
	// copied notes of the source release
	if n := rel["releaseNotes"].([]any); len(n) != 1 || n[0].(map[string]any)["text"] != "Three" {
		t.Errorf("notes = %v", n)
	}

	srv = tracksServer(t, map[string]string{"internal": internalTrack})
	_, err := newService(t, srv, io.Discard).Promote(t.Context(), PromoteOptions{Name: "v9", From: "internal", To: "beta", Status: StatusCompleted})
	if err == nil || clierr.ExitCode(err) != 1 || !strings.Contains(err.Error(), "v1, v3, v2") {
		t.Errorf("err = %v", err)
	}
	noMutations(t, srv)
}

func TestPromoteIdempotent(t *testing.T) {
	srv := tracksServer(t, map[string]string{
		"internal":   internalTrack,
		"production": `{"track":"production","releases":[{"name":"v3","status":"completed","versionCodes":["30","31"]}]}`,
	})
	var stderr strings.Builder
	res, err := newService(t, srv, &stderr).Promote(t.Context(), PromoteOptions{From: "internal", To: "production", Status: StatusCompleted})
	if err != nil {
		t.Fatal(err)
	}
	noMutations(t, srv)
	if !strings.Contains(stderr.String(), "already completed on production") {
		t.Errorf("stderr = %q", stderr.String())
	}
	if res.Track != "production" || len(res.Releases) != 1 {
		t.Errorf("res = %+v", res)
	}
	if c := strings.Join(calls(srv), ","); !strings.HasSuffix(c, "DELETE /edits/edit-1") {
		t.Errorf("edit not deleted: %s", c)
	}
}

func TestPromoteDropsNotesForUnlistedLanguages(t *testing.T) {
	// The listing has en-US and de-DE only.
	srv := tracksServer(t, map[string]string{"internal": `{"track":"internal","releases":[{"name":"v2","status":"completed","versionCodes":["20"],
		"releaseNotes":[{"language":"en-US","text":"Fixes"},{"language":"de-DE","text":"Korrekturen"},
		{"language":"es-419","text":"Arreglos"},{"language":"fr-FR","text":"Corrections"}]}]}`})
	var stderr strings.Builder
	res, err := newService(t, srv, &stderr).Promote(t.Context(), PromoteOptions{From: "internal", To: "production", Status: StatusCompleted})
	if err != nil {
		t.Fatal(err)
	}
	notes := sentReleases(t, srv)[0]["releaseNotes"].([]any)
	if len(notes) != 2 || notes[0].(map[string]any)["language"] != "de-DE" || notes[1].(map[string]any)["language"] != "en-US" {
		t.Errorf("notes = %v", notes)
	}
	for _, want := range []string{"es-MX", "fr-FR"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("no warning for %s: %q", want, stderr.String())
		}
	}
	if strings.Contains(stderr.String(), "en-US") || len(res.Releases[0].ReleaseNotes) != 2 {
		t.Errorf("stderr=%q res=%+v", stderr.String(), res)
	}
}

func TestPromoteNotesOverrideAndRollout(t *testing.T) {
	srv := tracksServer(t, map[string]string{"internal": internalTrack})
	_, err := newService(t, srv, io.Discard).Promote(t.Context(), PromoteOptions{
		From: "internal", To: "production", Status: StatusInProgress, UserFraction: 0.25,
		Notes: map[string]string{"de-DE": "Neu"},
	})
	if err != nil {
		t.Fatal(err)
	}
	rel := sentReleases(t, srv)[0]
	n := rel["releaseNotes"].([]any)
	if rel["status"] != "inProgress" || rel["userFraction"] != 0.25 || len(n) != 1 || n[0].(map[string]any)["text"] != "Neu" {
		t.Errorf("release = %v", rel)
	}
}

func TestPromoteNoReleases(t *testing.T) {
	srv := tracksServer(t, nil)
	_, err := newService(t, srv, io.Discard).Promote(t.Context(), PromoteOptions{From: "internal", To: "production", Status: StatusCompleted})
	if err == nil || !strings.Contains(err.Error(), "internal has no releases") {
		t.Errorf("err = %v", err)
	}
}

func TestSetReleaseRejectsBadStatus(t *testing.T) {
	srv := tracksServer(t, nil)
	s := newService(t, srv, io.Discard)
	for _, tt := range []struct {
		status string
		frac   float64
	}{{"bogus", 0}, {StatusInProgress, 0}, {StatusHalted, 0}, {StatusInProgress, 1}} {
		_, err := s.Promote(t.Context(), PromoteOptions{From: "internal", To: "production", Status: tt.status, UserFraction: tt.frac})
		if err == nil {
			t.Errorf("%s %v: no error", tt.status, tt.frac)
		}
	}
	noMutations(t, srv)
}

const stagedTrack = `{"track":"production","releases":[
	{"name":"v1","status":"completed","versionCodes":["10"]},
	{"name":"v2","status":"inProgress","userFraction":0.1,"versionCodes":["20"]}]}`

func TestRolloutNoStagedRelease(t *testing.T) {
	srv := tracksServer(t, map[string]string{"production": `{"track":"production","releases":[{"name":"v1","status":"completed","versionCodes":["10"]}]}`})
	_, err := newService(t, srv, io.Discard).SetRollout(t.Context(), "production", 0.5, CommitOptions{})
	if err == nil || clierr.ExitCode(err) != 1 || !strings.Contains(err.Error(), "no staged release") {
		t.Errorf("err = %v", err)
	}
	noMutations(t, srv)
	if !strings.HasSuffix(strings.Join(calls(srv), ","), "DELETE /edits/edit-1") {
		t.Error("edit not deleted")
	}
}

func TestRollout(t *testing.T) {
	srv := tracksServer(t, map[string]string{"production": stagedTrack})
	res, err := newService(t, srv, io.Discard).SetRollout(t.Context(), "production", 0.5, CommitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	rel := sentReleases(t, srv)
	if len(rel) != 2 || rel[0]["name"] != "v1" || rel[1]["userFraction"] != 0.5 || rel[1]["status"] != "inProgress" {
		t.Errorf("releases = %v", rel)
	}
	if res.Releases[1].UserFraction != 0.5 {
		t.Errorf("res = %+v", res)
	}
}

func TestComplete(t *testing.T) {
	srv := tracksServer(t, map[string]string{"production": stagedTrack})
	if _, err := newService(t, srv, io.Discard).CompleteRollout(t.Context(), "production", CommitOptions{}); err != nil {
		t.Fatal(err)
	}
	rel := sentReleases(t, srv)
	if len(rel) != 1 || rel[0]["name"] != "v2" || rel[0]["status"] != "completed" {
		t.Errorf("releases = %v", rel)
	}
	if _, ok := rel[0]["userFraction"]; ok {
		t.Errorf("userFraction in body: %v", rel[0])
	}
}

func TestHaltResume(t *testing.T) {
	srv := tracksServer(t, map[string]string{"production": stagedTrack})
	s := newService(t, srv, io.Discard)
	if _, err := s.Halt(t.Context(), "production", CommitOptions{}); err != nil {
		t.Fatal(err)
	}
	rel := sentReleases(t, srv)
	if len(rel) != 2 || rel[0]["status"] != "completed" || rel[1]["status"] != "halted" || rel[1]["userFraction"] != 0.1 {
		t.Errorf("halt: %v", rel)
	}

	halted := strings.Replace(stagedTrack, `"inProgress"`, `"halted"`, 1)
	srv = tracksServer(t, map[string]string{"production": halted})
	s = newService(t, srv, io.Discard)
	if _, err := s.Resume(t.Context(), "production", CommitOptions{}); err != nil {
		t.Fatal(err)
	}
	rel = sentReleases(t, srv)
	if rel[1]["status"] != "inProgress" || rel[1]["userFraction"] != 0.1 {
		t.Errorf("resume: %v", rel)
	}
	// nothing to halt on a halted release, nothing to resume on a staged one
	if _, err := s.Halt(t.Context(), "production", CommitOptions{}); err == nil {
		t.Error("halt of a halted release succeeded")
	}
	srv = tracksServer(t, map[string]string{"production": stagedTrack})
	if _, err := newService(t, srv, io.Discard).Resume(t.Context(), "production", CommitOptions{}); err == nil {
		t.Error("resume of an in-progress release succeeded")
	}
}

func TestSetReleaseNotes(t *testing.T) {
	srv := tracksServer(t, map[string]string{"production": `{"track":"production","releases":[
		{"name":"v1","status":"completed","versionCodes":["10"],"releaseNotes":[{"language":"de-DE","text":"Alt"}]},
		{"name":"v2","status":"inProgress","userFraction":0.1,"versionCodes":["20"]}]}`})
	var stderr strings.Builder
	rel, err := newService(t, srv, &stderr).SetReleaseNotes(t.Context(), "production", "v1",
		map[string]string{"en-US": "New", "fr-FR": "Nouveau"}, CommitOptions{})
	if err != nil {
		t.Fatal(err)
	}
	sent := sentReleases(t, srv)
	if len(sent) != 2 || sent[1]["name"] != "v2" {
		t.Errorf("other release not kept: %v", sent)
	}
	notes := sent[0]["releaseNotes"].([]any)
	if len(notes) != 2 || notes[0].(map[string]any)["text"] != "Alt" || notes[1].(map[string]any)["text"] != "New" {
		t.Errorf("notes = %v", notes)
	}
	if !strings.Contains(stderr.String(), "fr-FR") || rel.Name != "v1" || len(rel.ReleaseNotes) != 2 {
		t.Errorf("stderr=%q rel=%+v", stderr.String(), rel)
	}
}

func TestSetReleaseNotesDefaultsToNewest(t *testing.T) {
	srv := tracksServer(t, map[string]string{"internal": internalTrack})
	rel, err := newService(t, srv, io.Discard).SetReleaseNotes(t.Context(), "internal", "", map[string]string{"en-US": "X"}, CommitOptions{})
	if err != nil || rel.Name != "v3" {
		t.Fatalf("rel=%+v err=%v", rel, err)
	}
}

func TestTrackMutationsDryRun(t *testing.T) {
	dry := CommitOptions{DryRun: true}
	for name, run := range map[string]func(*Service) error{
		"promote": func(s *Service) error {
			_, err := s.Promote(t.Context(), PromoteOptions{From: "internal", To: "beta", Status: StatusCompleted, Commit: dry})
			return err
		},
		"rollout":  func(s *Service) error { _, err := s.SetRollout(t.Context(), "production", 0.3, dry); return err },
		"halt":     func(s *Service) error { _, err := s.Halt(t.Context(), "production", dry); return err },
		"complete": func(s *Service) error { _, err := s.CompleteRollout(t.Context(), "production", dry); return err },
		"notes": func(s *Service) error {
			_, err := s.SetReleaseNotes(t.Context(), "production", "", map[string]string{"en-US": "x"}, dry)
			return err
		},
	} {
		srv := tracksServer(t, map[string]string{"internal": internalTrack, "production": stagedTrack})
		if err := run(newService(t, srv, io.Discard)); err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		noMutations(t, srv)
	}
}

func TestListShowTracks(t *testing.T) {
	srv := tracksServer(t, map[string]string{"production": stagedTrack})
	srv.Handle("GET", editPath+"/tracks", jsonBody(`{"tracks":[`+internalTrack+`,`+stagedTrack+`]}`))
	s := newService(t, srv, io.Discard)
	list, err := s.ListTracks(t.Context())
	if err != nil || len(list) != 2 || list[1].Releases[1].UserFraction != 0.1 || list[0].Releases[1].VersionCodes[1] != "31" {
		t.Fatalf("list=%+v err=%v", list, err)
	}
	tr, err := s.ShowTrack(t.Context(), "production")
	if err != nil || tr.Track != "production" || len(tr.Releases) != 2 {
		t.Fatalf("track=%+v err=%v", tr, err)
	}
	noMutations(t, srv)
}
