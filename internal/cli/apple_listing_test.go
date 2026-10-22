package cli_test

import (
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/metadata"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
	"github.com/nikeee/store-cli/internal/testutil/golden"
)

func listingCLIServer(t *testing.T, infos, infoLocs, verLocs string) *ascfake.Server {
	t.Helper()
	srv := versionCLIServer(t, "app_appStoreVersions.json")
	srv.Handle("GET", "/v1/apps/{id}/appInfos", ascfake.JSON(http.StatusOK, infos))
	srv.Handle("GET", "/v1/appInfos/{id}/appInfoLocalizations", ascfake.JSON(http.StatusOK, infoLocs))
	srv.Handle("GET", "/v1/appStoreVersions/{id}/appStoreVersionLocalizations", ascfake.JSON(http.StatusOK, verLocs))
	srv.Handle("PATCH", "/v1/appInfoLocalizations/{id}", ascfake.JSON(http.StatusOK, "appInfoLocalization_created.json"))
	srv.Handle("PATCH", "/v1/appStoreVersionLocalizations/{id}", ascfake.JSON(http.StatusOK, "versionLocalization_created.json"))
	srv.Handle("POST", "/v1/appInfoLocalizations", ascfake.JSON(http.StatusCreated, "appInfoLocalization_created.json"))
	srv.Handle("POST", "/v1/appStoreVersionLocalizations", ascfake.JSON(http.StatusCreated, "versionLocalization_created.json"))
	srv.Handle("DELETE", "/v1/appInfoLocalizations/{id}", ascfake.JSON(http.StatusNoContent, "empty.json"))
	srv.Handle("DELETE", "/v1/appStoreVersionLocalizations/{id}", ascfake.JSON(http.StatusNoContent, "empty.json"))
	return srv
}

func defaultListingServer(t *testing.T) *ascfake.Server {
	return listingCLIServer(t, "app_appInfos.json", "appInfo_localizations_full.json", "version_localizations.json")
}

func TestAppleListingShowGolden(t *testing.T) {
	for _, mode := range []string{"text", "json", "toon"} {
		t.Run(mode, func(t *testing.T) {
			srv := defaultListingServer(t)
			code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "listing", "show", "-o", mode)
			if code != 0 || stderr != "" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			golden.Assert(t, "apple_listing_show."+mode, []byte(stdout))
		})
	}
}

func TestAppleListingShowLocale(t *testing.T) {
	srv := defaultListingServer(t)
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "listing", "show", "--locale", "de-DE", "-o", "json")
	if code != 0 || !strings.Contains(stdout, "Beispiel App") || strings.Contains(stdout, "en-US") {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	code, _, stderr = runEnv(t, buildEnv(srv), false, "apple", "listing", "show", "--locale", "ja")
	if code != 1 || !strings.Contains(stderr, "store apple listing locale add ja") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestAppleListingSetFlagsFromTable(t *testing.T) {
	// Every AppleFields entry with a flag is a flag of listing set (help lists them).
	srv := defaultListingServer(t)
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "listing", "set", "--help")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	n := 0
	for _, f := range metadata.AppleFields {
		if f.Flag == "" {
			if strings.Contains(stdout, "--whats-new") {
				t.Error("flag without table flag")
			}
			continue
		}
		n++
		if !strings.Contains(stdout, "--"+f.Flag+" ") {
			t.Errorf("flag --%s missing", f.Flag)
		}
	}
	if n == 0 {
		t.Fatal("no flags in table")
	}
}

func TestAppleListingSet(t *testing.T) {
	srv := defaultListingServer(t)
	dir := t.TempDir()
	file := filepath.Join(dir, "kw.txt")
	if err := os.WriteFile(file, []byte("one,two\n"), 0o600); err != nil {
		t.Fatal(err)
	}
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "listing", "set", "--locale", "en-US",
		"--subtitle", "New subtitle", "--keywords", "@"+file, "--marketing-url", "", "-o", "json")
	if code != 0 {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if !strings.Contains(stdout, `"subtitle": "New subtitle"`) || !strings.Contains(stdout, `"keywords": "one,two"`) {
		t.Errorf("stdout = %s", stdout)
	}
	var bodies []string
	for _, r := range srv.Requests() {
		if r.Method == "PATCH" {
			bodies = append(bodies, string(r.Body))
		}
	}
	if len(bodies) != 2 {
		t.Fatalf("bodies = %v", bodies)
	}
	if !strings.Contains(bodies[1], `"keywords":"one,two"`) || !strings.Contains(bodies[1], `"marketingUrl":null`) {
		t.Errorf("version body = %s", bodies[1])
	}
}

func TestAppleListingSetErrors(t *testing.T) {
	cases := []struct {
		name  string
		infos string
		args  []string
		code  int
		msg   string
	}{
		{"locked", "app_appInfos_locked.json", []string{"--name", "X"}, 1, "app metadata is locked"},
		{"too long", "app_appInfos.json", []string{"--subtitle", strings.Repeat("x", 31)}, 1, "en-US/subtitle.txt: 31 characters, limit 30"},
		{"no fields", "app_appInfos.json", nil, 2, "no field given"},
		{"no locale", "app_appInfos.json", []string{"--name", "X", "--locale", ""}, 2, ""},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			srv := listingCLIServer(t, c.infos, "appInfo_localizations_full.json", "version_localizations.json")
			args := append([]string{"apple", "listing", "set", "--locale", "en-US"}, c.args...)
			code, _, stderr := runEnv(t, buildEnv(srv), false, args...)
			if code != c.code || !strings.Contains(stderr, c.msg) {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			for _, r := range srv.Requests() {
				if r.Method != "GET" {
					t.Errorf("sent %s %s", r.Method, r.Path)
				}
			}
		})
	}
}

func TestAppleListingSetWithoutLocaleExits2(t *testing.T) {
	srv := listingCLIServer(t, "app_appInfos.json", "appInfo_localizations_full.json", "version_localizations.json")
	code, _, stderr := runEnv(t, buildEnv(srv), false, "apple", "listing", "set", "--name", "X")
	if code != 2 || !strings.Contains(stderr, "required flag --locale not set") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if n := len(srv.Requests()); n != 0 {
		t.Errorf("requests = %d", n)
	}
}

func TestAppleListingLocalePrimaryRefused(t *testing.T) {
	srv := defaultListingServer(t)
	code, _, stderr := runEnv(t, buildEnv(srv), false, "apple", "listing", "locale", "remove", "en-US")
	if code != 1 || !strings.Contains(stderr, "primary locale") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
}

func TestAppleListingLocaleAdd(t *testing.T) {
	srv := listingCLIServer(t, "app_appInfos.json", "appInfo_localizations_en_only.json", "version_localizations_en_only.json")
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "listing", "locale", "add", "fr-FR", "-o", "json")
	if code != 0 || !strings.Contains(stdout, `"action": "create"`) {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	posts := 0
	for _, r := range srv.Requests() {
		if r.Method == "POST" {
			posts++
		}
	}
	if posts != 2 {
		t.Errorf("posts = %d", posts)
	}
}

func TestAppleReleaseNotesSet(t *testing.T) {
	srv := defaultListingServer(t)
	code, stdout, stderr := runEnv(t, buildEnv(srv), false, "apple", "release-notes", "set", "1.3.0", "--locale", "de-DE", "--text", "Fehler behoben", "-o", "json")
	if code != 0 || !strings.Contains(stdout, `"whatsNew": "Fehler behoben"`) {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "en-US.txt"), []byte("Fixes\n"), 0o600)
	code, stdout, stderr = runEnv(t, buildEnv(srv), false, "apple", "release-notes", "set", "1.3.0", "--dir", dir, "-o", "json")
	if code != 0 || !strings.Contains(stdout, `"whatsNew": "Fixes"`) {
		t.Fatalf("code=%d stdout=%q stderr=%q", code, stdout, stderr)
	}
}

func TestAppleReleaseNotesUsage(t *testing.T) {
	srv := defaultListingServer(t)
	for _, args := range [][]string{
		{"1.3.0"},
		{"1.3.0", "--locale", "en-US"},
		{"1.3.0", "--dir", "x", "--locale", "en-US", "--text", "a"},
	} {
		code, _, stderr := runEnv(t, buildEnv(srv), false, append([]string{"apple", "release-notes", "set"}, args...)...)
		if code != 2 {
			t.Errorf("%v: code=%d stderr=%q", args, code, stderr)
		}
	}
}

func TestAppleListingMutationsDryRun(t *testing.T) {
	dir := t.TempDir()
	_ = os.WriteFile(filepath.Join(dir, "en-US.txt"), []byte("Fixes\n"), 0o600)
	for _, args := range [][]string{
		{"listing", "set", "--locale", "en-US", "--subtitle", "S", "--description", "D", "--marketing-url", ""},
		{"listing", "set", "--locale", "fr-FR", "--subtitle", "S", "--description", "D"},
		{"listing", "locale", "add", "fr-FR"},
		{"listing", "locale", "remove", "de-DE"},
		{"release-notes", "set", "1.3.0", "--locale", "en-US", "--text", "x"},
		{"release-notes", "set", "1.3.0", "--dir", dir},
	} {
		srv := defaultListingServer(t)
		code, stdout, stderr := runEnv(t, buildEnv(srv), false, append(append([]string{"apple"}, args...), "--dry-run", "-o", "json")...)
		if code != 0 || stdout == "" {
			t.Fatalf("%v: code=%d stdout=%q stderr=%q", args, code, stdout, stderr)
		}
		for _, r := range srv.Requests() {
			if r.Method != http.MethodGet {
				t.Errorf("%v: dry run sent %s %s", args, r.Method, r.Path)
			}
		}
	}
}
