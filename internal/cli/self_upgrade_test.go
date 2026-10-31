package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/nikeee/store-cli/internal/config"
)

const fakeTag = "v9.9.9"

var fakeArchive = fmt.Sprintf("store_9.9.9_%s_%s.tar.gz", runtime.GOOS, runtime.GOARCH)

func tarGz(t *testing.T, files map[string]string) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(zw)
	for name, content := range files {
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o755, Size: int64(len(content)), Typeflag: tar.TypeReg}); err != nil {
			t.Fatal(err)
		}
		if _, err := tw.Write([]byte(content)); err != nil {
			t.Fatal(err)
		}
	}
	if err := tw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes()
}

// releaseServer fakes the GitHub API and the downloads of release fakeTag.
// sum is the checksum listed for archive; "" lists its real one. It returns
// the number of archive downloads.
func releaseServer(t *testing.T, archive []byte, sum string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	if sum == "" {
		s := sha256.Sum256(archive)
		sum = hex.EncodeToString(s[:])
	}
	var downloads atomic.Int32
	mux := http.NewServeMux()
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	mux.HandleFunc("GET /repos/nikeee/store-cli/releases/latest", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{"tag_name":%q,"assets":[{"name":%q,"browser_download_url":%q},{"name":"checksums.txt","browser_download_url":%q}]}`,
			fakeTag, fakeArchive, srv.URL+"/dl/"+fakeArchive, srv.URL+"/dl/checksums.txt")
	})
	mux.HandleFunc("GET /dl/checksums.txt", func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, "%064d  store_9.9.9_plan9_mips.tar.gz\n%s  %s\n", 0, sum, fakeArchive)
	})
	mux.HandleFunc("GET /dl/"+fakeArchive, func(w http.ResponseWriter, _ *http.Request) {
		downloads.Add(1)
		_, _ = w.Write(archive)
	})
	return srv, &downloads
}

// installed writes an "old" binary into a temp dir and returns its resolved path.
func installed(t *testing.T) string {
	t.Helper()
	dir, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	exe := filepath.Join(dir, "store")
	if err := os.WriteFile(exe, []byte("old"), 0o750); err != nil {
		t.Fatal(err)
	}
	return exe
}

func upgradeWith(t *testing.T, apiURL, exe string, args ...string) (code int, res SelfUpgrade, stderr string) {
	t.Helper()
	var out, errb bytes.Buffer
	d := DefaultDeps()
	d.Stdout, d.Stderr, d.IsTTY, d.ErrTTY = &out, &errb, false, false
	d.Env = func(k string) (string, bool) { return apiURL, k == config.EnvInternalGitHubAPIURL }
	d.Executable = func() (string, error) { return exe, nil }
	code = Run(context.Background(), append([]string{"self-upgrade", "-o", "json"}, args...), d)
	if code == 0 {
		if err := json.Unmarshal(out.Bytes(), &res); err != nil {
			t.Fatalf("stdout %q: %v", out.String(), err)
		}
	}
	return code, res, errb.String()
}

func assertContent(t *testing.T, path, want string) {
	t.Helper()
	if got, err := os.ReadFile(path); err != nil || string(got) != want {
		t.Fatalf("%s = %q, %v; want %q", path, got, err, want)
	}
}

func TestSelfUpgradeReplacesTheBinary(t *testing.T) {
	srv, _ := releaseServer(t, tarGz(t, map[string]string{"LICENSE": "MIT", "store": "new"}), "")
	exe := installed(t)
	link := filepath.Join(t.TempDir(), "store")
	if err := os.Symlink(exe, link); err != nil {
		t.Fatal(err)
	}

	code, res, stderr := upgradeWith(t, srv.URL, link)
	want := SelfUpgrade{From: "dev", To: "9.9.9", Path: exe, Action: "upgrade"}
	if code != 0 || res != want {
		t.Fatalf("code=%d res=%+v stderr=%q", code, res, stderr)
	}
	assertContent(t, exe, "new")
	if fi, err := os.Stat(exe); err != nil || fi.Mode().Perm() != 0o750 {
		t.Fatalf("mode of %s: %v %v", exe, fi.Mode(), err)
	}
	if fi, err := os.Lstat(link); err != nil || fi.Mode()&os.ModeSymlink == 0 {
		t.Fatalf("symlink replaced: %v %v", fi.Mode(), err)
	}
	if entries, _ := os.ReadDir(filepath.Dir(exe)); len(entries) != 1 {
		t.Fatalf("temp file left over: %v", entries)
	}
}

func TestSelfUpgradeDryRunChangesNothing(t *testing.T) {
	srv, downloads := releaseServer(t, tarGz(t, map[string]string{"store": "new"}), "")
	exe := installed(t)
	code, res, stderr := upgradeWith(t, srv.URL, exe, "--dry-run")
	if code != 0 || res.Action != "upgrade" || res.To != "9.9.9" {
		t.Fatalf("code=%d res=%+v stderr=%q", code, res, stderr)
	}
	if downloads.Load() != 0 {
		t.Fatal("dry run downloaded the archive")
	}
	assertContent(t, exe, "old")
}

// The latest release or newer, a prerelease included, is never replaced.
func TestSelfUpgradeSkipsTheLatestReleaseOrNewer(t *testing.T) {
	prev := version
	t.Cleanup(func() { version = prev })
	srv, downloads := releaseServer(t, tarGz(t, map[string]string{"store": "new"}), "")
	exe := installed(t)
	for _, v := range []string{"9.9.9", "10.0.0-rc.1"} {
		version = v
		code, res, stderr := upgradeWith(t, srv.URL, exe)
		if code != 0 || res.Action != "skip" || downloads.Load() != 0 {
			t.Fatalf("%s: code=%d res=%+v downloads=%d stderr=%q", v, code, res, downloads.Load(), stderr)
		}
	}
	assertContent(t, exe, "old")
}

func TestSelfUpgradeDryRunFailsWithoutABuildForThisPlatform(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = fmt.Fprintf(w, `{"tag_name":%q,"assets":[]}`, fakeTag)
	}))
	t.Cleanup(srv.Close)
	code, _, stderr := upgradeWith(t, srv.URL, installed(t), "--dry-run")
	if code != 1 || !strings.Contains(stderr, "release v9.9.9 has no "+fakeArchive) {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestSelfUpgradeHintsSudoForAReadOnlyDir(t *testing.T) {
	if os.Geteuid() == 0 {
		t.Skip("root writes into read-only directories")
	}
	srv, _ := releaseServer(t, tarGz(t, map[string]string{"store": "new"}), "")
	exe := installed(t)
	dir := filepath.Dir(exe)
	if err := os.Chmod(dir, 0o555); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = os.Chmod(dir, 0o755) })
	code, _, stderr := upgradeWith(t, srv.URL, exe)
	if code != 1 || !strings.Contains(stderr, "sudo "+exe+" self-upgrade") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	assertContent(t, exe, "old")
}

func TestSelfUpgradeRejectsAChecksumMismatch(t *testing.T) {
	srv, _ := releaseServer(t, tarGz(t, map[string]string{"store": "evil"}), strings.Repeat("ab", 32))
	exe := installed(t)
	code, _, stderr := upgradeWith(t, srv.URL, exe)
	if code != 1 || !strings.Contains(stderr, "does not match its checksum") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	assertContent(t, exe, "old")
}
