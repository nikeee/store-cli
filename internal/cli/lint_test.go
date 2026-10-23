package cli_test

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"
)

// appleLintDir writes an App Store metadata directory with one iphone-6.9
// screenshot, plus files, and returns the metadata directory.
func appleLintDir(t *testing.T, files map[string]string) string {
	t.Helper()
	root := t.TempDir()
	tree := map[string]string{
		"metadata/en-US/name.txt":     "Holz",
		"metadata/en-US/keywords.txt": "timber,forest",
		"screenshots/en-US/01.png":    "1290x2796/1",
	}
	for p, c := range files {
		tree[p] = c
	}
	writeListingTree(t, root, tree)
	return filepath.Join(root, "metadata")
}

// lint needs no credentials and sends nothing.
func TestAppleLintOffline(t *testing.T) {
	code, stdout, stderr := runEnv(t, nil, false, "apple", "lint", appleLintDir(t, nil))
	if code != 0 || stdout != "no problems\n" || stderr != "" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	// no findings is an empty list, not null
	_, stdout, _ = runEnv(t, nil, false, "apple", "lint", appleLintDir(t, nil), "-o", "json")
	var res map[string]any
	if err := json.Unmarshal([]byte(stdout), &res); err != nil || res["findings"] == nil {
		t.Errorf("json = %q, %v", stdout, err)
	}
}

func TestAppleLintExitCodes(t *testing.T) {
	dir := appleLintDir(t, map[string]string{"metadata/en-US/subtitle.txt": strings.Repeat("x", 31)})
	code, stdout, stderr := runEnv(t, nil, false, "apple", "lint", dir)
	if want := filepath.Join(dir, "en-US", "subtitle.txt") + ":1:31: error: 31 characters, limit 30\n"; code != 1 || !strings.Contains(stdout, want) || stderr != "error: 1 error, 0 warnings\n" {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}

	warn := appleLintDir(t, map[string]string{"metadata/en-US/keywords.txt": "timber,wood app"})
	code, stdout, stderr = runEnv(t, nil, false, "apple", "lint", warn)
	if code != 0 || !strings.Contains(stdout, `warning: keyword "wood app" contains "app"`) || stderr != "" {
		t.Fatalf("warnings: code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	code, _, stderr = runEnv(t, nil, false, "apple", "lint", warn, "--strict")
	if code != 1 || stderr != "error: 0 errors, 1 warning (--strict)\n" {
		t.Fatalf("strict: code=%d stderr=%q", code, stderr)
	}

	if code, _, _ := runEnv(t, nil, false, "apple", "lint", dir, "--only", "nope"); code != 2 {
		t.Errorf("bad --only: code=%d", code)
	}
	if code, _, _ := runEnv(t, nil, false, "apple", "lint", dir, "--platform", "nope"); code != 2 {
		t.Errorf("bad --platform: code=%d", code)
	}
}

func TestGoogleLintJSON(t *testing.T) {
	dir := t.TempDir()
	writeListingTree(t, dir, map[string]string{
		"android/en-US/title.txt":       "Best Holz",
		"android/en-US/images/icon.png": "500x500/1",
	})
	notes := t.TempDir()
	writeListingTree(t, notes, map[string]string{"en-US.txt": strings.Repeat("x", 501)})
	code, stdout, _ := runEnv(t, nil, false, "google", "lint", dir, "--release-notes", notes, "-o", "json")
	var res struct {
		Findings []struct {
			Severity, File, Message string
			Line, Col               int
		}
		Errors, Warnings int
	}
	if err := json.Unmarshal([]byte(stdout), &res); err != nil || code != 1 {
		t.Fatalf("code=%d stdout=%q err=%v", code, stdout, err)
	}
	if res.Errors != 2 || res.Warnings != 3 || len(res.Findings) != 5 {
		t.Errorf("result = %+v", res)
	}
	if f := res.Findings[len(res.Findings)-1]; f.File != filepath.Join(notes, "en-US.txt") || f.Line != 1 || f.Col != 501 || f.Severity != "error" {
		t.Errorf("release notes finding = %+v", f)
	}
}

// On GitHub Actions every finding is one annotation: warnings from the
// command, errors from the failure.
func TestLintGitHubAnnotatesEachFindingOnce(t *testing.T) {
	ws := t.TempDir()
	dir := filepath.Join(ws, "metadata")
	writeListingTree(t, dir, map[string]string{
		"android/en-US/title.txt":                      "Holz!!",
		"android/en-US/images/featureGraphic.png":      "1000x500/1",
		"android/en-US/images/phoneScreenshots/01.png": "1080x1920/1",
		"android/en-US/images/phoneScreenshots/02.png": "1080x1920/2",
		"android/en-US/images/phoneScreenshots/03.png": "1080x1920/3",
		"android/en-US/images/phoneScreenshots/04.png": "1080x1920/4",
	})
	code, _, stderr := runEnv(t, githubEnv(ws, map[string]string{}), false, "google", "lint", dir)
	want := "::warning file=metadata/android/en-US/title.txt,line=1,col=5::repeated \"!\"; Google Play does not allow repeated special characters in the title\n" +
		"::error file=metadata/android/en-US/images/featureGraphic.png::1000x500, feature-graphic must be exactly 1024x500\n" +
		"error: 1 error, 1 warning\n"
	if code != 1 || stderr != want {
		t.Fatalf("code=%d stderr:\n%s\nwant:\n%s", code, stderr, want)
	}
}
