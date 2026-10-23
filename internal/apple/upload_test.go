package apple

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nikeee/store-cli/internal/apple/ascclient"
	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/output"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
	"github.com/nikeee/store-cli/internal/testutil/golden"
	"github.com/nikeee/store-cli/internal/testutil/ipafake"
)

// uploadService returns a Service whose clock advances by the slept time and
// the buffer that receives its stderr output.
func uploadService(t *testing.T, srv *ascfake.Server) (*Service, *bytes.Buffer) {
	t.Helper()
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	c, err := ascclient.New(ascfake.Credentials(), &http.Client{}, srv.URL, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	var stderr bytes.Buffer
	r := output.NewPrinter(output.ModeText, &bytes.Buffer{}, &stderr, false)
	sleep := func(_ context.Context, d time.Duration) error {
		now = now.Add(d)
		return nil
	}
	return NewService(c, r, Poll{Interval: time.Minute, Sleep: sleep}, func() time.Time { return now }), &stderr
}

// uploadServer serves the build upload flow; get answers GET /v1/buildUploads/{id}.
func uploadServer(t *testing.T, get ascfake.HandlerFunc) *ascfake.Server {
	t.Helper()
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/apps", ascfake.JSON(http.StatusOK, "apps_one.json"))
	srv.Handle("POST", "/v1/buildUploads", ascfake.JSON(http.StatusCreated, "buildUploads_create.json"))
	srv.Handle("POST", "/v1/buildUploadFiles", ascfake.JSON(http.StatusCreated, "buildUploadFiles_create.json"))
	srv.Handle("PATCH", "/v1/buildUploadFiles/{id}", ascfake.JSON(http.StatusOK, "buildUploadFiles_update.json"))
	srv.Handle("GET", "/v1/buildUploads/{id}", get)
	return srv
}

func testIPA(t *testing.T) string {
	t.Helper()
	return ipafake.Write(t, ipafake.Spec{BundleID: "com.example.app", Version: "1.3.0", Build: "45", Size: 2000})
}

func opts(path string) UploadOptions {
	return UploadOptions{Path: path, Platform: "ios", Timeout: 30 * time.Minute, NotesLocale: "en-US"}
}

func TestUploadBuildHappyPath(t *testing.T) {
	var polls atomic.Int32
	processing, complete := ascfake.JSON(http.StatusOK, "buildUploads_get_processing.json"), ascfake.JSON(http.StatusOK, "buildUploads_get_complete.json")
	srv := uploadServer(t, func(w http.ResponseWriter, r *http.Request) {
		if polls.Add(1) == 1 {
			processing(w, r)
			return
		}
		complete(w, r)
	})
	path := testIPA(t)
	svc, stderr := uploadService(t, srv)
	res, err := svc.UploadBuild(t.Context(), opts(path))
	if err != nil {
		t.Fatal(err)
	}

	var api, uploads []ascfake.Request
	for _, r := range srv.Requests() {
		if strings.HasPrefix(r.Path, "/upload/") {
			uploads = append(uploads, r)
		} else {
			api = append(api, r)
		}
	}
	var order []string
	for _, r := range api {
		order = append(order, r.Method+" "+r.Path)
	}
	wantOrder := []string{
		"GET /v1/apps", "POST /v1/buildUploads", "POST /v1/buildUploadFiles",
		"PATCH /v1/buildUploadFiles/buf-1", "GET /v1/buildUploads/bu-1", "GET /v1/buildUploads/bu-1",
	}
	if strings.Join(order, ",") != strings.Join(wantOrder, ",") {
		t.Errorf("requests = %v\nwant %v", order, wantOrder)
	}

	file, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if len(uploads) != 2 {
		t.Fatalf("%d upload requests", len(uploads))
	}
	for _, u := range uploads {
		want := file[:1000]
		if u.Path == "/upload/part2" {
			want = file[1000:]
		}
		if !bytes.Equal(u.Body, want) || u.Header.Get("Content-Type") != "application/octet-stream" {
			t.Errorf("%s: wrong body (%d bytes) or Content-Type %q", u.Path, len(u.Body), u.Header.Get("Content-Type"))
		}
		if a := u.Header.Get("Authorization"); a != "" {
			t.Errorf("%s carries Authorization %q", u.Path, a)
		}
	}

	var create struct {
		Data struct {
			Attributes    map[string]any
			Relationships map[string]struct{ Data struct{ Type, ID string } }
		}
	}
	if err := json.Unmarshal(api[1].Body, &create); err != nil {
		t.Fatal(err)
	}
	if a := create.Data.Attributes; a["cfBundleShortVersionString"] != "1.3.0" || a["cfBundleVersion"] != "45" || a["platform"] != "IOS" ||
		create.Data.Relationships["app"].Data.ID != "1234567890" || create.Data.Relationships["app"].Data.Type != "apps" {
		t.Errorf("buildUploads body = %s", api[1].Body)
	}
	var file2 struct {
		Data struct {
			Attributes    map[string]any
			Relationships map[string]struct{ Data struct{ Type, ID string } }
		}
	}
	if err := json.Unmarshal(api[2].Body, &file2); err != nil {
		t.Fatal(err)
	}
	if a := file2.Data.Attributes; a["fileName"] != "app.ipa" || a["fileSize"] != float64(2000) || a["uti"] != "com.apple.ipa" || a["assetType"] != "ASSET" ||
		file2.Data.Relationships["buildUpload"].Data.ID != "bu-1" || file2.Data.Relationships["buildUpload"].Data.Type != "buildUploads" {
		t.Errorf("buildUploadFiles body = %s", api[2].Body)
	}

	sum := md5.Sum(file)
	var patch struct {
		Data struct {
			ID         string
			Attributes struct {
				Uploaded            bool
				SourceFileChecksums struct {
					File struct{ Hash, Algorithm string }
				}
			}
		}
	}
	if err := json.Unmarshal(api[3].Body, &patch); err != nil {
		t.Fatal(err)
	}
	if a := patch.Data.Attributes; !a.Uploaded || a.SourceFileChecksums.File.Hash != hex.EncodeToString(sum[:]) || a.SourceFileChecksums.File.Algorithm != "MD5" || patch.Data.ID != "buf-1" {
		t.Errorf("checksum PATCH = %s", api[3].Body)
	}

	for _, want := range []string{"uploading app.ipa (", "upload accepted, waiting for Apple", "SDK version issue"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr lacks %q:\n%s", want, stderr)
		}
	}
	out, err := json.MarshalIndent(res, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	golden.Assert(t, "upload_result.json", append(out, '\n'))
}

func TestUploadBuildFailedState(t *testing.T) {
	srv := uploadServer(t, ascfake.JSON(http.StatusOK, "buildUploads_get_failed.json"))
	svc, _ := uploadService(t, srv)
	_, err := svc.UploadBuild(t.Context(), opts(testIPA(t)))
	if err == nil || clierr.ExitCode(err) != 1 {
		t.Fatalf("err = %v", err)
	}
	for _, want := range []string{"90062", "Invalid bundle: missing icon", "90189", "Redundant binary upload"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("error lacks %q: %v", want, err)
		}
	}
}

// Review Focus 2: builds take minutes to appear.
func TestUploadBuildWaitsForBuild(t *testing.T) {
	srv := uploadServer(t, ascfake.JSON(http.StatusOK, "buildUploads_get_complete.json"))
	var calls atomic.Int32
	empty, processing, valid := ascfake.JSON(http.StatusOK, "builds_empty.json"),
		ascfake.JSON(http.StatusOK, "builds_processing.json"), ascfake.JSON(http.StatusOK, "builds_valid.json")
	srv.Handle("GET", "/v1/builds", func(w http.ResponseWriter, r *http.Request) {
		switch calls.Add(1) {
		case 1, 2:
			empty(w, r)
		case 3, 4:
			processing(w, r)
		default:
			valid(w, r)
		}
	})
	srv.Handle("GET", "/v1/builds/{id}/betaBuildLocalizations", ascfake.JSON(http.StatusOK, "beta_build_localizations_empty.json"))
	srv.Handle("PATCH", "/v1/builds/{id}", ascfake.JSON(http.StatusOK, "build_updated.json"))
	var notesAtBuildCall atomic.Int32
	notes := ascfake.JSON(http.StatusCreated, "beta_build_localization.json")
	srv.Handle("POST", "/v1/betaBuildLocalizations", func(w http.ResponseWriter, r *http.Request) {
		notesAtBuildCall.Store(calls.Load())
		notes(w, r)
	})

	svc, stderr := uploadService(t, srv)
	o := opts(testIPA(t))
	o.Notes, o.Wait = "Fixed login", true
	no := false
	o.Encryption = &no
	res, err := svc.UploadBuild(t.Context(), o)
	if err != nil {
		t.Fatal(err)
	}
	if res.BuildID != "b3f1c2d4-0000-4a11-9c01-000000000045" || res.State != "complete" {
		t.Errorf("result = %+v", res)
	}
	if calls.Load() != 5 || notesAtBuildCall.Load() != 3 {
		t.Errorf("build lookups = %d, notes set after lookup %d; want 5 and 3", calls.Load(), notesAtBuildCall.Load())
	}
	for _, want := range []string{"waiting for build 45 (not visible yet", "waiting for build 45 (processing"} {
		if !strings.Contains(stderr.String(), want) {
			t.Errorf("stderr lacks %q:\n%s", want, stderr)
		}
	}
	var patched bool
	for _, r := range srv.Requests() {
		patched = patched || (r.Method == http.MethodPatch && strings.HasPrefix(r.Path, "/v1/builds/"))
	}
	if !patched {
		t.Error("encryption not set")
	}
}

func TestUploadBuildTimeout(t *testing.T) {
	srv := uploadServer(t, ascfake.JSON(http.StatusOK, "buildUploads_get_processing.json"))
	svc, stderr := uploadService(t, srv)
	o := opts(testIPA(t))
	o.Timeout = 5 * time.Minute
	_, err := svc.UploadBuild(t.Context(), o)
	if err == nil || clierr.ExitCode(err) != 1 ||
		!strings.Contains(err.Error(), "timed out after 5m0s waiting for Apple to accept the upload") || !strings.Contains(err.Error(), "PROCESSING") {
		t.Fatalf("err = %v", err)
	}
	if !strings.Contains(stderr.String(), "waiting for Apple to accept the upload (PROCESSING, 2m elapsed)") {
		t.Errorf("stderr = %s", stderr)
	}
}

// The command deadline also bounds the HTTP calls themselves.
func TestUploadBuildDeadlineWhileUploading(t *testing.T) {
	srv := uploadServer(t, ascfake.JSON(http.StatusOK, "buildUploads_get_complete.json"))
	srv.Handle("PUT", "/upload/{name}", func(w http.ResponseWriter, r *http.Request) {
		<-r.Context().Done()
	})
	svc, _ := uploadService(t, srv)
	o := opts(testIPA(t))
	o.Timeout = 200 * time.Millisecond
	_, err := svc.UploadBuild(t.Context(), o)
	if err == nil || !strings.Contains(err.Error(), "timed out after 200ms while uploading app.ipa") {
		t.Fatalf("err = %v", err)
	}
}

// The deadline starts before the app is resolved.
func TestUploadBuildDeadlineWhileResolving(t *testing.T) {
	srv := uploadServer(t, ascfake.JSON(http.StatusOK, "buildUploads_get_complete.json"))
	apps := ascfake.JSON(http.StatusOK, "apps_one.json")
	srv.Handle("GET", "/v1/apps", func(w http.ResponseWriter, r *http.Request) {
		select {
		case <-r.Context().Done():
		case <-time.After(time.Second):
			apps(w, r)
		}
	})
	svc, _ := uploadService(t, srv)
	o := opts(testIPA(t))
	o.Timeout = 100 * time.Millisecond
	_, err := svc.UploadBuild(t.Context(), o)
	if err == nil || !strings.Contains(err.Error(), "timed out after 100ms while resolving the app") {
		t.Fatalf("err = %v", err)
	}
}

// Hashing stops when the context ends.
func TestMD5SumStopsOnCancel(t *testing.T) {
	sum, err := md5Sum(t.Context(), strings.NewReader("abc"))
	if err != nil || sum != "900150983cd24fb0d6963f7d28e17f72" {
		t.Fatalf("sum = %s, err = %v", sum, err)
	}
	ctx, cancel := context.WithCancel(t.Context())
	cancel()
	if _, err := md5Sum(ctx, strings.NewReader("abc")); !errors.Is(err, context.Canceled) {
		t.Errorf("err = %v", err)
	}
}

func TestUploadBuildMismatch(t *testing.T) {
	for name, tc := range map[string]struct {
		app, platform, want string
	}{
		"app":      {"com.other.app", "ios", "IPA has bundle ID com.example.app, --app is com.other.app"},
		"platform": {"", "tvos", "IPA is for IOS, --platform is tvos"},
	} {
		t.Run(name, func(t *testing.T) {
			srv := ascfake.New(t)
			svc, _ := uploadService(t, srv)
			o := opts(testIPA(t))
			o.App, o.Platform = tc.app, tc.platform
			_, err := svc.UploadBuild(t.Context(), o)
			if err == nil || clierr.ExitCode(err) != 2 || err.Error() != tc.want {
				t.Fatalf("err = %v (exit %d)", err, clierr.ExitCode(err))
			}
			if n := len(srv.Requests()); n != 0 {
				t.Errorf("%d requests sent", n)
			}
		})
	}
}

func TestUploadBuildNotesTooLongSendsNothing(t *testing.T) {
	srv := ascfake.New(t)
	svc, _ := uploadService(t, srv)
	o := opts(testIPA(t))
	o.Notes = strings.Repeat("x", 4001)
	_, err := svc.UploadBuild(t.Context(), o)
	if err == nil || clierr.ExitCode(err) != 1 || len(srv.Requests()) != 0 {
		t.Fatalf("err = %v, %d requests", err, len(srv.Requests()))
	}
}

func TestUploadBuildNoAuthOnUploadURL(t *testing.T) {
	srv := uploadServer(t, ascfake.JSON(http.StatusOK, "buildUploads_get_complete.json"))
	svc, _ := uploadService(t, srv)
	if _, err := svc.UploadBuild(t.Context(), opts(testIPA(t))); err != nil {
		t.Fatal(err)
	}
	n := 0
	for _, r := range srv.Requests() {
		if strings.HasPrefix(r.Path, "/upload/") {
			n++
			if r.Header.Get("Authorization") != "" {
				t.Errorf("%s carries Authorization", r.Path)
			}
		}
	}
	if n != 2 {
		t.Errorf("%d upload requests", n)
	}
}

func TestPreviewUploadSendsNoChange(t *testing.T) {
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/apps", ascfake.JSON(http.StatusOK, "apps_one.json"))
	svc, _ := uploadService(t, srv)
	res, err := svc.PreviewUpload(t.Context(), opts(testIPA(t)))
	if err != nil {
		t.Fatal(err)
	}
	if res.State != "dry-run" || res.App != "com.example.app" || res.AppID != "1234567890" || res.Version != "1.3.0" || res.Build != "45" {
		t.Errorf("result = %+v", res)
	}
}

// Review Focus 2: the build never appears / never gets processed.
func TestUploadBuildWaitForBuildTimeout(t *testing.T) {
	for name, tc := range map[string]struct {
		fixture, want string
		wait          bool
	}{
		"never appears":   {"builds_empty.json", "waiting for build 45 to appear", false},
		"never processed": {"builds_processing.json", "waiting for build 45 to be processed", true},
	} {
		t.Run(name, func(t *testing.T) {
			srv := uploadServer(t, ascfake.JSON(http.StatusOK, "buildUploads_get_complete.json"))
			srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, tc.fixture))
			svc, stderr := uploadService(t, srv)
			o := opts(testIPA(t))
			o.Timeout, o.Wait = 5*time.Minute, tc.wait
			if !tc.wait {
				o.Notes = "Fixed login"
			}
			srv.Handle("GET", "/v1/builds/{id}/betaBuildLocalizations", ascfake.JSON(http.StatusOK, "beta_build_localizations_empty.json"))
			srv.Handle("POST", "/v1/betaBuildLocalizations", ascfake.JSON(http.StatusCreated, "beta_build_localization.json"))
			_, err := svc.UploadBuild(t.Context(), o)
			if err == nil || clierr.ExitCode(err) != 1 || !strings.Contains(err.Error(), "timed out after 5m0s") ||
				!strings.Contains(err.Error(), "build 45") || !strings.Contains(err.Error(), strings.TrimPrefix(tc.want, "waiting for build 45 ")) {
				t.Fatalf("err = %v", err)
			}
			if !strings.Contains(stderr.String(), "waiting for build 45") {
				t.Errorf("stderr = %s", stderr)
			}
		})
	}
}

// A step after Apple accepted the upload fails: the error says the build was
// uploaded and the hint names the commands that finish the remaining steps.
func TestUploadBuildStepAfterUploadFails(t *testing.T) {
	const uploaded = "build 45 (1.3.0) was uploaded (build upload bu-1); then: "
	for name, tc := range map[string]struct {
		builds    string // GET /v1/builds fixture
		failNotes bool
		failPatch bool
		wait      bool
		want      string // in the message after uploaded
		hint      string
		status    int
	}{
		"never appears": {
			builds: "builds_empty.json", want: "timed out after 5m0s waiting for build 45 to appear",
			hint: "store apple testflight notes set 45 --version 1.3.0 --platform ios --locale en-US --text <notes> && " +
				"store apple build set-encryption 45 false --version 1.3.0 --platform ios && store apple build wait 45 --version 1.3.0 --platform ios",
		},
		"notes fail": {
			builds: "builds_processing.json", failNotes: true, want: "set notes of build 45", status: http.StatusConflict,
			hint: "store apple testflight notes set 45 --version 1.3.0 --platform ios --locale en-US --text <notes> && " +
				"store apple build set-encryption 45 false --version 1.3.0 --platform ios && store apple build wait 45 --version 1.3.0 --platform ios",
		},
		"encryption fails": {
			builds: "builds_processing.json", failPatch: true, want: "An attribute value is invalid.", status: http.StatusConflict,
			hint: "store apple build set-encryption 45 false --version 1.3.0 --platform ios && store apple build wait 45 --version 1.3.0 --platform ios",
		},
		"never processed": {
			builds: "builds_processing.json", want: "timed out after 5m0s waiting for build 45 to be processed",
			hint: "store apple build wait 45 --version 1.3.0 --platform ios",
		},
		// Nothing is left to do once processing failed: no hint to wait.
		"processing failed": {
			builds: "builds_failed.json", want: "build 45 ended failed in App Store Connect",
		},
	} {
		t.Run(name, func(t *testing.T) {
			srv := uploadServer(t, ascfake.JSON(http.StatusOK, "buildUploads_get_complete.json"))
			srv.Handle("GET", "/v1/builds", ascfake.JSON(http.StatusOK, tc.builds))
			srv.Handle("GET", "/v1/builds/{id}/betaBuildLocalizations", ascfake.JSON(http.StatusOK, "beta_build_localizations_empty.json"))
			notes := ascfake.JSON(http.StatusCreated, "beta_build_localization.json")
			if tc.failNotes {
				notes = ascfake.JSON(http.StatusConflict, "error_409.json")
			}
			srv.Handle("POST", "/v1/betaBuildLocalizations", notes)
			patch := ascfake.JSON(http.StatusOK, "build_updated.json")
			if tc.failPatch {
				patch = ascfake.JSON(http.StatusConflict, "error_409.json")
			}
			srv.Handle("PATCH", "/v1/builds/{id}", patch)
			svc, _ := uploadService(t, srv)
			o := opts(testIPA(t))
			no := false
			o.Timeout, o.Notes, o.Encryption, o.Wait = 5*time.Minute, "Fixed login", &no, true
			res, err := svc.UploadBuild(t.Context(), o)
			var ce *clierr.Error
			if !errors.As(err, &ce) || clierr.ExitCode(err) != 1 {
				t.Fatalf("err = %v", err)
			}
			if !strings.HasPrefix(ce.Message, uploaded) || !strings.Contains(ce.Message, tc.want) {
				t.Errorf("message = %q", ce.Message)
			}
			if ce.Hint != tc.hint || ce.Status != tc.status {
				t.Errorf("hint = %q, status = %d", ce.Hint, ce.Status)
			}
			if res.BuildUploadID != "bu-1" || res.State != "complete" {
				t.Errorf("result = %+v", res)
			}
		})
	}
}
