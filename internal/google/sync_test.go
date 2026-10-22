package google

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/metadata"
	"github.com/nikeee/store-cli/internal/testutil/pngfake"
)

// writeTree writes files (path relative to root -> content) below root; a
// path ending in .png gets an opaque PNG of the size in the content ("WxH").
func writeTree(t *testing.T, root string, files map[string]string) {
	t.Helper()
	for p, c := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if strings.HasSuffix(p, ".png") {
			var w, h int
			if _, err := fmt.Sscanf(c, "%dx%d", &w, &h); err != nil {
				t.Fatal(err)
			}
			pngfake.Write(t, full, w, h, 1)
			continue
		}
		if err := os.WriteFile(full, []byte(c), 0o600); err != nil {
			t.Fatal(err)
		}
	}
}

// problemDir is a metadata directory with one problem per locale.
func problemDir(t *testing.T) (dir, alpha, icon string) {
	dir = t.TempDir()
	writeTree(t, dir, map[string]string{
		"android/de-DE/title.txt":                      strings.Repeat("ü", 31),
		"android/en-US/images/phoneScreenshots/02.png": "640x1280",
		"android/fr-FR/images/phoneScreenshots/01.png": "640x1280",
		"android/fr-FR/images/icon.png":                "500x500",
		"android/en-US/title.txt":                      "Fine",
	})
	alpha = filepath.Join(dir, "android", "en-US", "images", "phoneScreenshots", "01.png")
	pngfake.WriteAlpha(t, alpha, 640, 1280)
	return dir, alpha, filepath.Join(dir, "android", "fr-FR", "images", "icon.png")
}

func TestReadPushDirReportsAllProblems(t *testing.T) {
	dir, alpha, icon := problemDir(t)
	_, err := ReadPushDir(PushOptions{Dir: dir})
	if err == nil || clierr.ExitCode(err) != 1 {
		t.Fatalf("err = %v", err)
	}
	for _, want := range []string{
		"de-DE/title.txt: 31 characters, limit 30",
		"en-US: " + alpha + ": has an alpha channel",
		"fr-FR: phone needs at least 2 screenshots, got 1",
		"fr-FR: " + icon + ": 500x500, icon must be exactly 512x512",
	} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q: %v", want, err)
		}
	}
}

func TestReadPushDirOnly(t *testing.T) {
	dir, _, _ := problemDir(t)
	_, err := ReadPushDir(PushOptions{Dir: dir, Only: []string{"texts", "screenshots"}})
	if err == nil || clierr.ExitCode(err) != 2 || !strings.Contains(err.Error(), `"screenshots"`) {
		t.Fatalf("err = %v", err)
	}
	// parts that are not pushed are not validated
	_, err = ReadPushDir(PushOptions{Dir: dir, Only: []string{"texts"}})
	if err == nil || !strings.Contains(err.Error(), "title.txt") || strings.Contains(err.Error(), "phone") {
		t.Errorf("texts: err = %v", err)
	}
	_, err = ReadPushDir(PushOptions{Dir: dir, Only: []string{"images"}})
	if err == nil || strings.Contains(err.Error(), "title.txt") || !strings.Contains(err.Error(), "phone") {
		t.Errorf("images: err = %v", err)
	}
}

// A folder named by the App Store code would fail only at Play.
func TestReadPushDirWantsPlayCodes(t *testing.T) {
	dir := t.TempDir()
	writeTree(t, dir, map[string]string{"android/zh-Hans/title.txt": "App", "android/en-US/title.txt": "App"})
	_, err := ReadPushDir(PushOptions{Dir: dir})
	if err == nil || clierr.ExitCode(err) != 1 || err.Error() != "zh-Hans: Google Play calls this language zh-CN, rename the folder" {
		t.Fatalf("err = %v", err)
	}
}

func TestReadPushDirNeedsAndroidFolder(t *testing.T) {
	dir := t.TempDir()
	_, err := ReadPushDir(PushOptions{Dir: dir})
	if err == nil || clierr.ExitCode(err) != 1 || !strings.Contains(err.Error(), filepath.Join(dir, "android")+" not found") {
		t.Fatalf("err = %v", err)
	}
}

func TestInitScaffolds(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "metadata")
	// App Store locales are written as Play codes; it and it-IT are one folder
	res, err := Init(dir, []string{"zh-Hans", "it", "it-IT"})
	if err != nil {
		t.Fatal(err)
	}
	folders := []string{"images", "images/phoneScreenshots", "images/sevenInchScreenshots", "images/tenInchScreenshots",
		"images/tvScreenshots", "images/wearScreenshots"}
	var want []string
	for _, l := range []string{"zh-CN", "it-IT"} {
		for _, f := range metadata.GoogleFields {
			want = append(want, filepath.Join(dir, "android", l, f.File))
		}
		for _, f := range folders {
			want = append(want, filepath.Join(dir, "android", l, filepath.FromSlash(f)))
		}
	}
	if !slices.Equal(res.Created, want) || len(res.Existing) != 0 {
		t.Fatalf("created %v\nwant %v\nexisting %v", res.Created, want, res.Existing)
	}
	for _, p := range want {
		if _, err := os.Stat(p); err != nil {
			t.Error(err)
		}
	}
	for _, l := range []string{"zh-Hans", "it"} {
		if _, err := os.Stat(filepath.Join(dir, "android", l)); !os.IsNotExist(err) {
			t.Errorf("%s created: %v", l, err)
		}
	}

	// a second run leaves everything alone
	res, err = Init(dir, []string{"it-IT"})
	if err != nil || len(res.Created) != 0 || len(res.Existing) != len(metadata.GoogleFields)+len(folders) {
		t.Errorf("rerun: res=%+v err=%v", res, err)
	}
}

func TestInitRejectsBadLocale(t *testing.T) {
	tmp := t.TempDir()
	_, err := Init(filepath.Join(tmp, "metadata"), []string{"en-US", "../x"})
	if err == nil || clierr.ExitCode(err) != 2 || !strings.Contains(err.Error(), `--locale "../x"`) {
		t.Fatalf("err = %v", err)
	}
	if entries, err := os.ReadDir(tmp); err != nil || len(entries) != 0 {
		t.Errorf("created %v (err=%v)", entries, err)
	}
}
