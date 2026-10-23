package google

import (
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/lint"
)

// healthyDir returns a metadata directory without findings.
func healthyDir(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"android/en-US/title.txt":                      "Holz Tracker",
		"android/en-US/short_description.txt":          "Track your wood",
		"android/en-US/full_description.txt":           "A wood tracker.",
		"android/en-US/images/phoneScreenshots/01.png": "1080x1920",
		"android/en-US/images/phoneScreenshots/02.png": "1080x1920",
		"android/en-US/images/phoneScreenshots/03.png": "1920x1080",
		"android/en-US/images/phoneScreenshots/04.png": "1242x2208",
		"android/en-US/images/featureGraphic.png":      "1024x500",
		"android/en-US/images/icon.png":                "512x512",
	})
	return dir
}

func runLint(t *testing.T, o PushOptions, notesDir string) *lint.Result {
	t.Helper()
	r, err := Lint(o, notesDir)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func errorsOf(r *lint.Result) []lint.Finding {
	var out []lint.Finding
	for _, f := range r.Findings {
		if f.Severity == lint.Error {
			out = append(out, f)
		}
	}
	return out
}

func TestLintHealthyDirHasNoFindings(t *testing.T) {
	if r := runLint(t, PushOptions{Dir: healthyDir(t)}, ""); len(r.Findings) != 0 {
		t.Errorf("findings: %+v", r.Findings)
	}
}

// Everything push rejects is an error.
func TestLintReportsPushProblems(t *testing.T) {
	dir, alpha, icon := problemDir(t)
	got := errorsOf(runLint(t, PushOptions{Dir: dir}, ""))
	want := []lint.Finding{
		{Severity: lint.Error, Message: "fr-FR: phone needs at least 2 screenshots, got 1"},
		{Severity: lint.Error, File: filepath.Join(dir, "android", "de-DE", "title.txt"), Line: 1, Col: 31, Message: "31 characters, limit 30"},
		{Severity: lint.Error, File: alpha, Message: "has an alpha channel, Google Play rejects it for phone"},
		{Severity: lint.Error, File: icon, Message: "500x500, icon must be exactly 512x512"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("errors:\n%+v\nwant:\n%+v", got, want)
	}
}

func TestLintPresence(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"android/en-US/images/icon.png": "512x512"})
	r := runLint(t, PushOptions{Dir: dir}, "")
	want := []lint.Finding{
		{Severity: lint.Warning, Message: "no feature graphic; Google Play requires one to publish"},
		{Severity: lint.Warning, Message: "no locale has 2 screenshots; Google Play requires at least 2 to publish"},
	}
	if !reflect.DeepEqual(r.Findings, want) {
		t.Errorf("findings: %+v", r.Findings)
	}

	// a directory without images manages none, so nothing is missing
	texts := t.TempDir()
	writeTree(t, texts, map[string]string{"android/en-US/title.txt": "Holz"})
	if r := runLint(t, PushOptions{Dir: texts}, ""); len(r.Findings) != 0 {
		t.Errorf("texts only: %+v", r.Findings)
	}
}

// Play promotes apps with 4 screenshots of at least 1080 px in 9:16 or 16:9.
func TestLintPhoneRecommendations(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{
		"android/en-US/images/phoneScreenshots/01.png": "1080x1920",
		"android/en-US/images/phoneScreenshots/02.png": "1080x2160",
		"android/en-US/images/phoneScreenshots/03.png": "720x1280",
		"android/en-US/images/featureGraphic.png":      "1024x500",
	})
	shots := filepath.Join(dir, "android", "en-US", "images", "phoneScreenshots")
	r := runLint(t, PushOptions{Dir: dir}, "")
	want := []lint.Finding{
		{Severity: lint.Warning, Message: "en-US: only 3 phone screenshots; Google Play promotes apps with at least 4"},
		{Severity: lint.Warning, File: filepath.Join(shots, "02.png"), Message: "1080x2160; Google Play promotes apps with 9:16 or 16:9 screenshots of at least 1080 px"},
		{Severity: lint.Warning, File: filepath.Join(shots, "03.png"), Message: "720x1280; Google Play promotes apps with 9:16 or 16:9 screenshots of at least 1080 px"},
	}
	if !reflect.DeepEqual(r.Findings, want) {
		t.Errorf("findings:\n%+v\nwant:\n%+v", r.Findings, want)
	}
}

func TestTitleIssues(t *testing.T) {
	tests := []struct {
		title string
		want  []clierr.Issue
	}{
		{"Holz 🌲 Tracker", []clierr.Issue{{Line: 1, Col: 6, Message: `emoji or emoticon "🌲"; Google Play does not allow them in the title`}}},
		{"Holz :)", []clierr.Issue{{Line: 1, Col: 6, Message: `emoji or emoticon ":)"; Google Play does not allow them in the title`}}},
		{"HOLZ TRACKER", []clierr.Issue{{Line: 1, Message: "all capital letters; Google Play allows them only in brand names"}}},
		{"Holz!!", []clierr.Issue{{Line: 1, Col: 5, Message: `repeated "!"; Google Play does not allow repeated special characters in the title`}}},
		{"#1 Holz", []clierr.Issue{{Line: 1, Col: 1, Message: `"#1" suggests ranking or a promotion; Google Play does not allow it in the title`}}},
		{"Best Holz", []clierr.Issue{{Line: 1, Col: 1, Message: `"Best" suggests ranking or a promotion; Google Play does not allow it in the title`}}},
		{"Holz free", []clierr.Issue{{Line: 1, Col: 6, Message: `"free" suggests ranking or a promotion; Google Play does not allow it in the title`}}},
		{"Holz 50% off", []clierr.Issue{{Line: 1, Col: 8, Message: `"% off" suggests ranking or a promotion; Google Play does not allow it in the title`}}},
	}
	for _, tt := range tests {
		if got := titleIssues("", tt.title); !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q:\n%+v\nwant:\n%+v", tt.title, got, tt.want)
		}
	}
	for _, quiet := range []string{"Holz", "Wait...", "FreeCell", "Tasks:Plan", "Ad Blocker", "BBC News", "TV", "Holz™", "A-Z Notes", "Top 10 Recipes #10"} {
		if got := titleIssues("", quiet); len(got) != 0 {
			t.Errorf("%q: %+v", quiet, got)
		}
	}
	if got := titleIssues("t.txt", "HOLZ!!"); len(got) != 2 || got[0].File != "t.txt" {
		t.Errorf("file: %+v", got)
	}
}

func TestLintTitlesAreWarnings(t *testing.T) {
	dir := healthyDir(t)
	writeTree(t, dir, map[string]string{"android/en-US/title.txt": "Best Holz"})
	r := runLint(t, PushOptions{Dir: dir}, "")
	want := []lint.Finding{{Severity: lint.Warning, File: filepath.Join(dir, "android", "en-US", "title.txt"), Line: 1, Col: 1,
		Message: `"Best" suggests ranking or a promotion; Google Play does not allow it in the title`}}
	if !reflect.DeepEqual(r.Findings, want) {
		t.Errorf("findings: %+v", r.Findings)
	}
}

func TestLintReleaseNotes(t *testing.T) {
	notes := t.TempDir()
	writeTree(t, notes, map[string]string{"en-US.txt": strings.Repeat("x", 501)})
	r := runLint(t, PushOptions{Dir: healthyDir(t)}, notes)
	want := []lint.Finding{{Severity: lint.Error, File: filepath.Join(notes, "en-US.txt"), Line: 1, Col: 501, Message: "501 characters, limit 500"}}
	if !reflect.DeepEqual(r.Findings, want) {
		t.Errorf("findings: %+v", r.Findings)
	}

	// two files for one Play language
	writeTree(t, notes, map[string]string{"en-US.txt": "ok", "es-MX.txt": "uno", "es-419.txt": "dos"})
	if r := runLint(t, PushOptions{Dir: healthyDir(t)}, notes); r.Errors != 1 || r.Findings[0].File != "" {
		t.Errorf("findings: %+v", r.Findings)
	}
}

func TestLintOnlyTexts(t *testing.T) {
	dir, _, _ := problemDir(t)
	got := runLint(t, PushOptions{Dir: dir, Only: []string{"texts"}}, "")
	if len(got.Findings) != 1 || got.Findings[0].Message != "31 characters, limit 30" {
		t.Errorf("findings: %+v", got.Findings)
	}
}

func TestLintMissingDir(t *testing.T) {
	if _, err := Lint(PushOptions{Dir: t.TempDir()}, ""); err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v", err)
	}
}
