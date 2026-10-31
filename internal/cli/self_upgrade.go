package cli

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/spf13/cobra"
	"golang.org/x/mod/semver"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/config"
)

const (
	releaseRepo      = "nikeee/store-cli"
	defaultGitHubAPI = "https://api.github.com"
)

// SelfUpgrade is the result of `store self-upgrade`.
type SelfUpgrade struct {
	From   string `json:"from"`
	To     string `json:"to"`
	Path   string `json:"path"`
	Action string `json:"action"`
}

// githubRelease is the part of a GitHub release that self-upgrade reads.
type githubRelease struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
	} `json:"assets"`
}

func (r githubRelease) assetURL(name string) (string, error) {
	for _, a := range r.Assets {
		if a.Name == name {
			return a.URL, nil
		}
	}
	return "", clierr.New("release %s has no %s", r.TagName, name)
}

func newSelfUpgradeCmd(Deps) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "self-upgrade",
		Short: "Replace this binary with the latest release",
		Long: "Downloads the archive of the latest GitHub release for this platform, verifies it against the\n" +
			"release's checksums.txt and replaces this binary with the store in it. Skips when this binary\n" +
			"already is the latest release or newer; local builds (dev) are always replaced.\n" +
			"Result: SelfUpgrade",
		Example: "  store self-upgrade --dry-run\n  store self-upgrade",
		Args:    cobra.NoArgs,
		RunE:    runSelfUpgrade,
	}
	addDryRun(cmd)
	return cmd
}

func runSelfUpgrade(cmd *cobra.Command, _ []string) error {
	d := cmdDeps(cmd)
	exe, err := d.Executable()
	if err != nil {
		return clierr.New("locate this binary: %v", err)
	}
	// Replace the file a symlink points to, not the symlink.
	if exe, err = filepath.EvalSymlinks(exe); err != nil {
		return clierr.New("locate this binary: %v", err)
	}

	api := defaultGitHubAPI
	if v, ok := d.Env(config.EnvInternalGitHubAPIURL); ok && v != "" {
		api = v
	}
	ctx, client := cmd.Context(), httpClient(cmd)
	body, err := fetch(ctx, client, api+"/repos/"+releaseRepo+"/releases/latest")
	if err != nil {
		return err
	}
	var rel githubRelease
	if err := json.Unmarshal(body, &rel); err != nil {
		return clierr.New("latest release: %v", err)
	}

	latest := strings.TrimPrefix(rel.TagName, "v")
	res := SelfUpgrade{From: version, To: latest, Path: exe, Action: "upgrade"}
	// dev is no semver: local builds always upgrade.
	if semver.IsValid("v"+version) && semver.Compare("v"+latest, "v"+version) <= 0 {
		res.Action = "skip"
		return emit(cmd, res)
	}
	archive := fmt.Sprintf("store_%s_%s_%s.tar.gz", latest, runtime.GOOS, runtime.GOARCH)
	archiveURL, err := rel.assetURL(archive)
	if err != nil {
		return err
	}
	if dryRun(cmd) {
		return emit(cmd, res)
	}

	bin, err := downloadStore(cmd, client, rel, archive, archiveURL)
	if err != nil {
		return err
	}
	if err := replaceFile(exe, bin); err != nil {
		e := clierr.New("replace %s: %v", exe, err)
		if errors.Is(err, fs.ErrPermission) {
			e.Hint = "sudo " + exe + " self-upgrade"
		}
		return e
	}
	return emit(cmd, res)
}

// downloadStore downloads archive of rel from archiveURL, verifies it against
// the release's checksums.txt and returns the store binary in it.
func downloadStore(cmd *cobra.Command, client *http.Client, rel githubRelease, archive, archiveURL string) ([]byte, error) {
	ctx := cmd.Context()
	sumsURL, err := rel.assetURL("checksums.txt")
	if err != nil {
		return nil, err
	}
	sums, err := fetch(ctx, client, sumsURL)
	if err != nil {
		return nil, err
	}
	want, ok := checksumOf(sums, archive)
	if !ok {
		return nil, clierr.New("%s is not listed in checksums.txt of %s", archive, rel.TagName)
	}

	printer(cmd).Progressf("downloading %s", archive)
	data, err := fetch(ctx, client, archiveURL)
	if err != nil {
		return nil, err
	}
	if got := sha256.Sum256(data); hex.EncodeToString(got[:]) != want {
		return nil, clierr.New("%s does not match its checksum in checksums.txt", archive)
	}
	bin, err := storeFromArchive(data)
	if err != nil {
		return nil, clierr.New("%s: %v", archive, err)
	}
	return bin, nil
}

func fetch(ctx context.Context, client *http.Client, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode != http.StatusOK {
		return nil, clierr.New("GET %s: %s", url, resp.Status)
	}
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, clierr.New("GET %s: %v", url, err)
	}
	return b, nil
}

// checksumOf returns the SHA-256 of name in a checksums.txt ("<sum>  <name>" lines).
func checksumOf(sums []byte, name string) (string, bool) {
	for line := range strings.Lines(string(sums)) {
		if f := strings.Fields(line); len(f) == 2 && f[1] == name {
			return f[0], true
		}
	}
	return "", false
}

// storeFromArchive returns the file store of a release tar.gz.
func storeFromArchive(data []byte) ([]byte, error) {
	zr, err := gzip.NewReader(bytes.NewReader(data))
	if err != nil {
		return nil, err
	}
	tr := tar.NewReader(zr)
	for {
		h, err := tr.Next()
		if errors.Is(err, io.EOF) {
			return nil, errors.New("no store binary in the archive")
		}
		if err != nil {
			return nil, err
		}
		if h.Name == "store" && h.Typeflag == tar.TypeReg {
			return io.ReadAll(tr)
		}
	}
}

// replaceFile replaces path with content and keeps its mode. The rename is
// atomic: path is never half written.
func replaceFile(path string, content []byte) error {
	fi, err := os.Stat(path)
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(path), ".store-upgrade-*")
	if err != nil {
		return err
	}
	defer func() { _ = os.Remove(tmp.Name()) }() // fails harmlessly after the rename
	if _, err := tmp.Write(content); err != nil {
		_ = tmp.Close()
		return err
	}
	// On disk before the rename, so a crash cannot leave an empty binary.
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(fi.Mode().Perm()); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}
