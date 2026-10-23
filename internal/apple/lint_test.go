package apple

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/lint"
	"github.com/nikeee/store-cli/internal/testutil/pngfake"
)

// healthyDir writes a metadata directory without findings and returns it.
// Its screenshots are in <root>/screenshots.
func healthyDir(t *testing.T) (root, dir string) {
	t.Helper()
	root = t.TempDir()
	writeTree(t, root, map[string]string{
		"metadata/en-US/name.txt":        "Holz",
		"metadata/en-US/subtitle.txt":    "Track your wood",
		"metadata/en-US/description.txt": "A wood tracker.",
		"metadata/en-US/keywords.txt":    "timber,forest,lumber",
		"metadata/en-US/support_url.txt": "https://example.com/support",
	})
	shot(t, root, "en-US/01.png", 1320, 2868)
	shot(t, root, "en-US/ipad-13/01.png", 2064, 2752)
	return root, filepath.Join(root, "metadata")
}

// shot writes an opaque screenshot of w x h below <root>/screenshots.
func shot(t *testing.T, root, rel string, w, h int) string {
	t.Helper()
	p := filepath.Join(root, "screenshots", filepath.FromSlash(rel))
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		t.Fatal(err)
	}
	pngfake.Write(t, p, w, h, 1)
	return p
}

func runLint(t *testing.T, o PushOptions, notesDir string) *lint.Result {
	t.Helper()
	if o.Platform == "" {
		o.Platform = "ios"
	}
	r, err := Lint(o, notesDir)
	if err != nil {
		t.Fatal(err)
	}
	return r
}

func TestLintHealthyDirHasNoFindings(t *testing.T) {
	_, dir := healthyDir(t)
	if r := runLint(t, PushOptions{Dir: dir}, ""); len(r.Findings) != 0 {
		t.Errorf("findings: %+v", r.Findings)
	}
}

// Everything push rejects is an error, with its place in the file.
func TestLintReportsPushProblems(t *testing.T) {
	root, dir := healthyDir(t)
	writeTree(t, dir, map[string]string{"de-DE/subtitle.txt": strings.Repeat("ü", 31), "de-DE/keywords.txt": strings.Repeat("ä", 51)})
	alpha := filepath.Join(root, "screenshots", "en-US", "02.png")
	pngfake.WriteAlpha(t, alpha, 1320, 2868)
	r := runLint(t, PushOptions{Dir: dir}, "")
	want := []lint.Finding{
		{Severity: lint.Error, File: filepath.Join(dir, "de-DE", "keywords.txt"), Line: 1, Col: 51, Message: "102 bytes, limit 100"},
		{Severity: lint.Error, File: filepath.Join(dir, "de-DE", "subtitle.txt"), Line: 1, Col: 31, Message: "31 characters, limit 30"},
		{Severity: lint.Error, File: alpha, Message: "has an alpha channel, App Store Connect rejects it"},
	}
	if !reflect.DeepEqual(r.Findings, want) {
		t.Errorf("findings:\n%+v\nwant:\n%+v", r.Findings, want)
	}
}

func TestLintRequiredFiles(t *testing.T) {
	_, dir := healthyDir(t)
	writeTree(t, dir, map[string]string{"en-US/description.txt": "\n", "en-US/support_url.txt": ""})
	r := runLint(t, PushOptions{Dir: dir}, "")
	want := []lint.Finding{
		{Severity: lint.Error, File: filepath.Join(dir, "en-US", "description.txt"), Message: "must not be empty, App Store Connect requires it to submit"},
		{Severity: lint.Error, File: filepath.Join(dir, "en-US", "support_url.txt"), Message: "must not be empty, App Store Connect requires it to submit"},
	}
	if !reflect.DeepEqual(r.Findings, want) {
		t.Errorf("findings: %+v", r.Findings)
	}
}

// The file of the less used Apple Watch size is the error.
func TestLintWatchSizes(t *testing.T) {
	root, dir := healthyDir(t)
	shot(t, root, "en-US/w1.png", 416, 496)
	shot(t, root, "en-US/w2.png", 416, 496)
	odd := shot(t, root, "de-DE/w1.png", 396, 484)
	r := runLint(t, PushOptions{Dir: dir}, "")
	want := []lint.Finding{{Severity: lint.Error, File: odd,
		Message: "396x484, the other Apple Watch screenshots are 416x496; App Store Connect needs one size in all locales"}}
	if !reflect.DeepEqual(r.Findings, want) {
		t.Errorf("findings: %+v", r.Findings)
	}
}

func TestLintDevicePresence(t *testing.T) {
	root := t.TempDir()
	dir := filepath.Join(root, "metadata")
	writeTree(t, dir, map[string]string{"en-US/name.txt": "Holz"})
	shot(t, root, "en-US/01.png", 1668, 2388) // ipad-11
	r := runLint(t, PushOptions{Dir: dir}, "")
	want := []lint.Finding{
		{Severity: lint.Warning, Message: "no iphone-6.9 or iphone-6.5 screenshots; App Store Connect requires one of them if the app runs on iPhone"},
		{Severity: lint.Warning, Message: "iPad screenshots but no ipad-13 ones; App Store Connect requires ipad-13 if the app runs on iPad"},
	}
	if !reflect.DeepEqual(r.Findings, want) {
		t.Errorf("findings: %+v", r.Findings)
	}

	// no iPhone rule for a Mac app, none at all without screenshots
	mac := t.TempDir()
	writeTree(t, filepath.Join(mac, "metadata"), map[string]string{"en-US/name.txt": "Holz"})
	shot(t, mac, "en-US/01.png", 2880, 1800)
	if r := runLint(t, PushOptions{Dir: filepath.Join(mac, "metadata"), Platform: "macos"}, ""); len(r.Findings) != 0 {
		t.Errorf("macos: %+v", r.Findings)
	}
	if r := runLint(t, PushOptions{Dir: dir, ScreenshotsDir: t.TempDir()}, ""); len(r.Findings) != 0 {
		t.Errorf("no screenshots: %+v", r.Findings)
	}
}

func TestKeywordIssues(t *testing.T) {
	tests := []struct {
		kw   string
		want []clierr.Issue
	}{
		{"timber,forest", nil},
		{"写真,AI,3d,ok", []clierr.Issue{
			{Line: 1, Col: 1, Message: `keyword "写真" is shorter than 3 characters`},
			{Line: 1, Col: 4, Message: `keyword "AI" is shorter than 3 characters`},
			{Line: 1, Col: 7, Message: `keyword "3d" is shorter than 3 characters`},
			{Line: 1, Col: 10, Message: `keyword "ok" is shorter than 3 characters`},
		}},
		{"timber, forest ,lumber", []clierr.Issue{
			{Line: 1, Col: 8, Message: "spaces next to commas use 2 of the 100 bytes"},
		}},
		{"Forest,timber,forest", []clierr.Issue{
			{Line: 1, Col: 15, Message: `keyword "forest" appears twice`},
		}},
		{"log app,holzig", []clierr.Issue{
			{Line: 1, Col: 1, Message: `keyword "log app" contains "app", which does not help search`},
		}},
		{"tracker,holz log,diary", []clierr.Issue{
			{Line: 1, Col: 1, Message: `keyword "tracker" repeats "tracker" from the subtitle, which is searchable already`},
			{Line: 1, Col: 9, Message: `keyword "holz log" repeats "holz" from the name, which is searchable already`},
		}},
		{"timber,,forest,", nil},
	}
	for _, tt := range tests {
		got := keywordIssues("", tt.kw, "Holz", "Wood Tracker")
		if !reflect.DeepEqual(got, tt.want) {
			t.Errorf("%q:\n%+v\nwant:\n%+v", tt.kw, got, tt.want)
		}
	}
	// "app" in the name too is one warning, not two
	want := []clierr.Issue{{Line: 1, Col: 1, Message: `keyword "app" contains "app", which does not help search`}}
	if got := keywordIssues("", "app", "Holz App", ""); !reflect.DeepEqual(got, want) {
		t.Errorf("app in name: %+v", got)
	}
	if got := keywordIssues("k.txt", "ab", "", ""); len(got) != 1 || got[0].File != "k.txt" {
		t.Errorf("file: %+v", got)
	}
}

func TestLintReleaseNotes(t *testing.T) {
	_, dir := healthyDir(t)
	notes := t.TempDir()
	writeTree(t, notes, map[string]string{"en-US.txt": strings.Repeat("x", 4001)})
	r := runLint(t, PushOptions{Dir: dir}, notes)
	want := []lint.Finding{{Severity: lint.Error, File: filepath.Join(notes, "en-US.txt"), Line: 1, Col: 4001, Message: "4001 characters, limit 4000"}}
	if !reflect.DeepEqual(r.Findings, want) {
		t.Errorf("findings: %+v", r.Findings)
	}
}

// --only texts reads no screenshots, so no screenshot rule fires.
func TestLintOnlyTexts(t *testing.T) {
	root, dir := healthyDir(t)
	pngfake.WriteAlpha(t, filepath.Join(root, "screenshots", "en-US", "02.png"), 1320, 2868)
	if r := runLint(t, PushOptions{Dir: dir, Only: []string{"texts"}}, ""); len(r.Findings) != 0 {
		t.Errorf("findings: %+v", r.Findings)
	}
}

func TestLintMissingDir(t *testing.T) {
	_, err := Lint(PushOptions{Dir: filepath.Join(t.TempDir(), "nope"), Platform: "ios"}, "")
	if err == nil || !strings.Contains(err.Error(), "not found") {
		t.Errorf("err = %v", err)
	}
}
