package cli_test

import (
	"io"
	"net/http"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/testutil/golden"
	"github.com/nikeee/store-cli/internal/testutil/playfake"
	"github.com/nikeee/store-cli/internal/testutil/pngfake"
)

const (
	editImagesPath       = googleEditsPath + "/edit-1/listings/{language}/{imageType}"
	editImagesUploadPath = "/upload/androidpublisher/v3/applications/{packageName}/edits/edit-1/listings/{language}/{imageType}"
)

// playImagesServer serves two phone screenshots for it-IT and nothing else.
func playImagesServer(t *testing.T) *playfake.Server {
	srv := googleEditServer(t)
	srv.Handle("GET", editImagesPath, func(w http.ResponseWriter, r *http.Request) {
		if r.PathValue("language") == "it-IT" && r.PathValue("imageType") == "phoneScreenshots" {
			_, _ = io.WriteString(w, `{"images":[
				{"id":"img1","url":"https://play.example/img1","sha256":"aa11"},
				{"id":"img2","url":"https://play.example/img2","sha256":"bb22"}]}`)
			return
		}
		_, _ = io.WriteString(w, `{}`)
	})
	srv.Handle("DELETE", editImagesPath, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"deleted":[{"id":"img1"},{"id":"img2"}]}`)
	})
	srv.Handle("POST", editImagesUploadPath, func(w http.ResponseWriter, _ *http.Request) {
		_, _ = io.WriteString(w, `{"image":{"id":"new"}}`)
	})
	return srv
}

func imageArgs(args ...string) []string {
	return append([]string{"google", "images"}, append(args, "--app", "com.example.app")...)
}

func writeShots(t *testing.T, w, h, n int) []string {
	t.Helper()
	dir := t.TempDir()
	var out []string
	for i := range n {
		p := filepath.Join(dir, string(rune('a'+i))+".png")
		pngfake.Write(t, p, w, h, uint8(i+1))
		out = append(out, p)
	}
	return out
}

func TestGoogleImagesListGolden(t *testing.T) {
	for _, mode := range []string{"text", "json", "toon"} {
		t.Run(mode, func(t *testing.T) {
			srv := playImagesServer(t)
			code, stdout, stderr := runEnv(t, googleEnv(srv, nil), false, append(imageArgs("list", "--locale", "it-IT"), "-o", mode)...)
			if code != 0 || stderr != "" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			golden.Assert(t, "google_images_list."+mode, []byte(stdout))
		})
	}
}

func TestGoogleImagesUploadGolden(t *testing.T) {
	files := writeShots(t, 640, 1280, 2)
	for _, mode := range []string{"text", "json", "toon"} {
		t.Run(mode, func(t *testing.T) {
			srv := playImagesServer(t)
			args := append(imageArgs("upload", "--locale", "it-IT", "--type", "phone", "--replace"), files...)
			code, stdout, stderr := runEnv(t, googleEnv(srv, nil), false, append(args, "-o", mode)...)
			if code != 0 || stderr != "it-IT phone: image 1 of 2\nit-IT phone: image 2 of 2\n" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			golden.Assert(t, "google_images_upload."+mode, []byte(stdout))
		})
	}
}

func TestGoogleImagesDeleteGolden(t *testing.T) {
	for _, mode := range []string{"text", "json", "toon"} {
		t.Run(mode, func(t *testing.T) {
			srv := playImagesServer(t)
			code, stdout, stderr := runEnv(t, googleEnv(srv, nil), false, append(imageArgs("delete", "--locale", "it-IT", "--type", "phone"), "-o", mode)...)
			if code != 0 || stderr != "" {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			golden.Assert(t, "google_images_delete."+mode, []byte(stdout))
		})
	}
}

func TestGoogleImagesUsageErrors(t *testing.T) {
	files := writeShots(t, 640, 1280, 2)
	tests := []struct {
		name string
		args []string
		want string
	}{
		{"list without locale", imageArgs("list"), "--locale"},
		{"upload without type", append(imageArgs("upload", "--locale", "en-US"), files...), "--type"},
		{"unknown type", append(imageArgs("upload", "--locale", "en-US", "--type", "poster"), files...), "feature-graphic"},
		{"bad locale", imageArgs("delete", "--locale", "xx_YY!", "--type", "phone"), "--locale"},
		{"delete without type", imageArgs("delete", "--locale", "en-US"), "--type"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			srv := playImagesServer(t)
			code, _, stderr := runEnv(t, googleEnv(srv, nil), false, tt.args...)
			if code != 2 || !strings.Contains(stderr, tt.want) {
				t.Fatalf("code=%d stderr=%q", code, stderr)
			}
			if n := len(srv.Requests()); n != 0 {
				t.Errorf("%d requests", n)
			}
		})
	}
}

func TestGoogleImagesValidationExits1WithoutRequests(t *testing.T) {
	bad := writeShots(t, 100, 100, 2) // below the 320 px minimum
	srv := playImagesServer(t)
	code, _, stderr := runEnv(t, googleEnv(srv, nil), false, append(imageArgs("upload", "--locale", "en-US", "--type", "phone"), bad...)...)
	if code != 1 || !strings.Contains(stderr, "at least 320") {
		t.Fatalf("code=%d stderr=%q", code, stderr)
	}
	if n := len(srv.Requests()); n != 0 {
		t.Errorf("%d requests", n)
	}
}

func TestGoogleImagesMutationsDryRun(t *testing.T) {
	files := writeShots(t, 640, 1280, 2)
	for _, args := range [][]string{
		append(imageArgs("upload", "--locale", "it-IT", "--type", "phone"), files...),
		append(imageArgs("upload", "--locale", "it-IT", "--type", "phone", "--replace"), files...),
		imageArgs("delete", "--locale", "it-IT", "--type", "phone"),
	} {
		srv := playImagesServer(t)
		code, stdout, stderr := runEnv(t, googleEnv(srv, nil), false, append(args, "--dry-run", "-o", "json")...)
		if code != 0 || !strings.Contains(stdout, `"changes"`) || !strings.Contains(stdout, `"images"`) {
			t.Fatalf("%v: code=%d stdout=%q stderr=%q", args, code, stdout, stderr)
		}
		for _, r := range srv.Requests() {
			switch {
			case r.Method == http.MethodGet:
			case r.Method == http.MethodPost && r.Path == "/androidpublisher/v3/applications/com.example.app/edits":
			case r.Method == http.MethodDelete && r.Path == "/androidpublisher/v3/applications/com.example.app/edits/edit-1":
			default:
				t.Errorf("%v: dry run sent %s %s", args, r.Method, r.Path)
			}
		}
	}
}
