package cli_test

import (
	"path/filepath"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/config"
	"github.com/nikeee/store-cli/internal/testutil/filetree"
)

func githubEnv(ws string, env map[string]string) map[string]string {
	env[config.EnvGitHubActions] = "true"
	env[config.EnvGitHubWorkspace] = ws
	return env
}

// On GitHub Actions every problem of a push is an annotation at its place,
// the file named relative to the workspace.
func TestGitHubAnnotatesPushProblems(t *testing.T) {
	ws := t.TempDir()
	dir := filepath.Join(ws, "metadata")
	filetree.Write(t, dir, map[string]string{
		"android/de-DE/title.txt":       "Titel\n" + strings.Repeat("ü", 30),
		"android/en-US/images/icon.png": "500x500/1",
	})
	srv := newPlayStore(t)
	code, _, stderr := runEnv(t, githubEnv(ws, googleEnv(srv.Server, nil)), false, pushArgs(dir)...)
	want := "::error file=metadata/android/de-DE/title.txt,line=2,col=25::36 characters, limit 30\n" +
		"::error file=metadata/android/en-US/images/icon.png::500x500, icon must be exactly 512x512\n"
	if code != 1 || !strings.HasPrefix(stderr, want) || !strings.Contains(stderr, "error: de-DE/title.txt: 36 characters, limit 30; ") {
		t.Fatalf("code=%d stderr:\n%s", code, stderr)
	}
}

func TestGitHubAnnotatesReleaseNotes(t *testing.T) {
	ws := t.TempDir()
	dir := filepath.Join(ws, "release-notes")
	filetree.Write(t, dir, map[string]string{"en-US.txt": "New:\n- " + strings.Repeat("x", 500) + "\n"})
	srv := newPlayStore(t)
	args := []string{"google", "release-notes", "set", "internal", "--dir", dir, "--app", "com.example.app", "-o", "json"}
	code, _, stderr := runEnv(t, githubEnv(ws, googleEnv(srv.Server, nil)), false, args...)
	if want := "::error file=release-notes/en-US.txt,line=2,col=496::507 characters, limit 500\n"; code != 1 || !strings.HasPrefix(stderr, want) {
		t.Fatalf("code=%d stderr:\n%s", code, stderr)
	}
	if n := len(srv.Requests()); n != 0 {
		t.Errorf("%d requests sent", n)
	}
}

func TestGitHubAnnotatesOnlyOnGitHub(t *testing.T) {
	code, _, stderr := runEnv(t, map[string]string{config.EnvGitHubActions: "false"}, false, "nope")
	if code != 2 || strings.Contains(stderr, "::") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	code, _, stderr = runEnv(t, githubEnv("", map[string]string{}), false, "nope")
	if code != 2 || !strings.HasPrefix(stderr, `::error::unknown command "nope" for "store"`) {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}
