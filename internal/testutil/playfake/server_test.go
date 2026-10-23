package playfake_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"sync"
	"testing"

	"golang.org/x/oauth2"
	"google.golang.org/api/androidpublisher/v3"
	"google.golang.org/api/googleapi"
	"google.golang.org/api/option"

	"github.com/nikeee/store-cli/internal/testutil/playfake"
)

// recorder captures the failures the fake reports instead of failing the test.
type recorder struct {
	testing.TB
	mu   sync.Mutex
	errs []string
}

func (r *recorder) Errorf(format string, args ...any) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.errs = append(r.errs, fmt.Sprintf(format, args...))
}

var auth = map[string]string{"Authorization": "Bearer " + playfake.AccessToken}

func send(t *testing.T, method, url string, header map[string]string, body string) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader(body))
	if err != nil {
		t.Fatal(err)
	}
	for k, v := range header {
		req.Header.Set(k, v)
	}
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func readBody(t *testing.T, resp *http.Response) string {
	t.Helper()
	b, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

const editsPath = "/androidpublisher/v3/applications/{packageName}/edits"

func TestServer(t *testing.T) {
	rec := &recorder{TB: t}
	srv := playfake.New(rec)
	srv.Handle("GET", editsPath+"/{editId}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "generic "+r.PathValue("editId"))
	})
	srv.Handle("GET", editsPath+"/{editId}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "latest "+r.PathValue("packageName")+" "+r.PathValue("editId"))
	})
	srv.Handle("POST", editsPath, playfake.JSON(http.StatusOK, "edit.json"))

	if resp := send(t, "GET", srv.URL+"/androidpublisher/v3/applications/com.example.app/edits/e1", auth, ""); readBody(t, resp) != "latest com.example.app e1" {
		t.Errorf("placeholder route: %d", resp.StatusCode)
	}
	if body := readBody(t, send(t, "POST", srv.URL+"/androidpublisher/v3/applications/com.example.app/edits", auth, "{}")); !strings.Contains(body, `"id": "edit-1"`) {
		t.Errorf("fixture: %s", body)
	}
	if last := srv.Requests()[1]; last.Method != "POST" || last.Path != "/androidpublisher/v3/applications/com.example.app/edits" ||
		string(last.Body) != "{}" || last.Header.Get("Authorization") == "" {
		t.Errorf("recorded %+v", last)
	}

	tests := []struct {
		name, method, url string
		header            map[string]string
		code              int
		failure           string
	}{
		{"unregistered", "DELETE", srv.URL + "/androidpublisher/v3/applications/com.example.app/edits/e1", auth, http.StatusNotImplemented,
			"no handler for DELETE /androidpublisher/v3/applications/com.example.app/edits/e1; registered: GET " + editsPath + "/{editId}, GET " +
				editsPath + "/{editId}, POST " + editsPath},
		{"no token", "POST", srv.URL + "/androidpublisher/v3/applications/com.example.app/edits", nil, http.StatusUnauthorized,
			"POST /androidpublisher/v3/applications/com.example.app/edits without the fake access token"},
		{"wrong token", "POST", srv.URL + "/androidpublisher/v3/applications/com.example.app/edits", map[string]string{"Authorization": "Bearer other"},
			http.StatusUnauthorized, "without the fake access token"},
		{"unknown upload type", "POST", srv.URL + "/upload/androidpublisher/v3/applications/com.example.app/edits/e1/bundles?uploadType=chunked", auth,
			http.StatusBadRequest, `uploadType "chunked"`},
		{"unknown upload session", "PUT", srv.URL + "/upload/androidpublisher/v3/applications/com.example.app/edits/e1/bundles?upload_id=nope", auth,
			http.StatusNotFound, `unknown upload session "nope"`},
	}
	for _, tt := range tests {
		rec.errs = nil
		resp := send(t, tt.method, tt.url, tt.header, "")
		if resp.StatusCode != tt.code || len(rec.errs) != 1 || !strings.Contains(rec.errs[0], tt.failure) {
			t.Errorf("%s: code=%d failures=%q", tt.name, resp.StatusCode, rec.errs)
		}
	}
}

func TestServerToken(t *testing.T) {
	rec := &recorder{TB: t}
	srv := playfake.New(rec)
	claims := base64.RawURLEncoding.EncodeToString([]byte(`{"scope":"` + androidpublisher.AndroidpublisherScope + `"}`))
	form := url.Values{"grant_type": {"urn:ietf:params:oauth:grant-type:jwt-bearer"}, "assertion": {"e30." + claims + ".sig"}}
	resp := send(t, "POST", srv.URL+"/token", map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, form.Encode())
	var tok struct {
		AccessToken string `json:"access_token"`
		TokenType   string `json:"token_type"`
	}
	if err := json.Unmarshal([]byte(readBody(t, resp)), &tok); err != nil || tok.AccessToken != playfake.AccessToken || tok.TokenType != "Bearer" {
		t.Errorf("token: %d %+v %v", resp.StatusCode, tok, err)
	}
	if n := len(srv.Requests()); n != 0 || len(rec.errs) != 0 {
		t.Errorf("token request recorded: %d, failures %q", n, rec.errs)
	}

	other := base64.RawURLEncoding.EncodeToString([]byte(`{"scope":"https://www.googleapis.com/auth/cloud-platform"}`))
	for name, f := range map[string]url.Values{
		"grant type": {"grant_type": {"client_credentials"}, "assertion": form["assertion"]},
		"scope":      {"grant_type": form["grant_type"], "assertion": {"e30." + other + ".sig"}},
	} {
		rec.errs = nil
		resp := send(t, "POST", srv.URL+"/token", map[string]string{"Content-Type": "application/x-www-form-urlencoded"}, f.Encode())
		if resp.StatusCode != http.StatusBadRequest || len(rec.errs) != 1 {
			t.Errorf("%s: code=%d failures=%q", name, resp.StatusCode, rec.errs)
		}
	}
}

const bundlesPath = "/upload/androidpublisher/v3/applications/{packageName}/edits/{editId}/bundles"

// mediaHandler records the media the fake hands to the upload route.
func mediaHandler(got *[]byte, method *string) playfake.HandlerFunc {
	h := playfake.JSON(http.StatusOK, "bundle.json")
	return func(w http.ResponseWriter, r *http.Request) {
		*got, _ = io.ReadAll(r.Body)
		*method = r.Method
		h(w, r)
	}
}

func TestServerResumableUpload(t *testing.T) {
	srv := playfake.New(t)
	var media []byte
	var method string
	srv.Handle("POST", bundlesPath, mediaHandler(&media, &method))
	start := send(t, "POST", srv.URL+"/upload/androidpublisher/v3/applications/com.example.app/edits/e1/bundles?uploadType=resumable", auth, "")
	loc, err := url.Parse(start.Header.Get("Location"))
	if start.StatusCode != http.StatusOK || err != nil || loc.Query().Get("upload_id") == "" || loc.Path != "/upload/androidpublisher/v3/applications/com.example.app/edits/e1/bundles" {
		t.Fatalf("start: %d %q", start.StatusCode, start.Header.Get("Location"))
	}

	chunk := func(rng, body string, extra map[string]string) *http.Response {
		h := map[string]string{"Content-Range": rng}
		for k, v := range auth {
			h[k] = v
		}
		for k, v := range extra {
			h[k] = v
		}
		return send(t, "PUT", loc.String(), h, body)
	}
	if resp := chunk("bytes 0-2/*", "abc", nil); resp.StatusCode != http.StatusPermanentRedirect || resp.Header.Get("Range") != "bytes=0-2" {
		t.Errorf("chunk 1: %d %q", resp.StatusCode, resp.Header.Get("Range"))
	}
	resp := chunk("bytes 3-4/*", "de", map[string]string{"X-GUploader-No-308": "yes"})
	if resp.StatusCode != http.StatusOK || resp.Header.Get("X-HTTP-Status-Code-Override") != "308" || resp.Header.Get("Range") != "bytes=0-4" {
		t.Errorf("chunk 2 without 308: %d %v", resp.StatusCode, resp.Header)
	}
	if media != nil {
		t.Fatalf("route called before the upload completed: %q", media)
	}
	resp = chunk("bytes 5-5/6", "f", nil)
	if body := readBody(t, resp); resp.StatusCode != http.StatusOK || !strings.Contains(body, `"versionCode": 42`) {
		t.Errorf("final chunk: %d %s", resp.StatusCode, body)
	}
	if string(media) != "abcdef" || method != "POST" {
		t.Errorf("route got %s %q", method, media)
	}
	if n := len(srv.Requests()); n != 4 {
		t.Errorf("%d requests recorded, want 4", n)
	}
}

func TestServerSimpleUploads(t *testing.T) {
	srv := playfake.New(t)
	var media []byte
	var method string
	srv.Handle("POST", bundlesPath, mediaHandler(&media, &method))
	base := srv.URL + "/upload/androidpublisher/v3/applications/com.example.app/edits/e1/bundles"

	if resp := send(t, "POST", base+"?uploadType=media", auth, "raw bytes"); resp.StatusCode != http.StatusOK || string(media) != "raw bytes" {
		t.Errorf("media: %d %q", resp.StatusCode, media)
	}

	var body bytes.Buffer
	mw := multipart.NewWriter(&body)
	meta, _ := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/json"}})
	_, _ = io.WriteString(meta, "{}")
	part, _ := mw.CreatePart(textproto.MIMEHeader{"Content-Type": {"application/octet-stream"}})
	_, _ = io.WriteString(part, "multipart bytes")
	_ = mw.Close()
	h := map[string]string{"Content-Type": "multipart/related; boundary=" + mw.Boundary()}
	for k, v := range auth {
		h[k] = v
	}
	if resp := send(t, "POST", base+"?uploadType=multipart", h, body.String()); resp.StatusCode != http.StatusOK || string(media) != "multipart bytes" {
		t.Errorf("multipart: %d %q", resp.StatusCode, media)
	}
}

// TestServerWithClient runs the official client's uploads against the fake:
// a single-chunk (multipart) and a chunked (resumable) upload.
func TestServerWithClient(t *testing.T) {
	srv := playfake.New(t)
	var media []byte
	var method string
	srv.Handle("POST", bundlesPath, mediaHandler(&media, &method))
	client := oauth2.NewClient(t.Context(), oauth2.StaticTokenSource(&oauth2.Token{AccessToken: playfake.AccessToken}))
	api, err := androidpublisher.NewService(t.Context(), option.WithHTTPClient(client), option.WithEndpoint(srv.URL+"/"))
	if err != nil {
		t.Fatal(err)
	}
	for name, size := range map[string]int{"multipart": 10, "resumable": 2*googleapi.MinUploadChunkSize + 7} {
		data := bytes.Repeat([]byte{'x'}, size)
		b, err := api.Edits.Bundles.Upload("com.example.app", "e1").
			Media(bytes.NewReader(data), googleapi.ChunkSize(googleapi.MinUploadChunkSize), googleapi.ContentType("application/octet-stream")).
			Context(t.Context()).Do()
		if err != nil || b.VersionCode != 42 || !bytes.Equal(media, data) {
			t.Errorf("%s: bundle=%+v err=%v got %d bytes", name, b, err, len(media))
		}
		if q := srv.Requests()[len(srv.Requests())-1].Query.Get("uploadType"); q != name {
			t.Errorf("%s: uploadType=%q", name, q)
		}
		if method != "POST" {
			t.Errorf("%s: handler got %s", name, method)
		}
	}
	// the real client asks for 200 instead of 308 on resumable chunks, which
	// the fake must honor
	no308 := false
	for _, r := range srv.Requests() {
		no308 = no308 || r.Header.Get("X-GUploader-No-308") == "yes"
	}
	if !no308 {
		t.Error("no chunk carried X-GUploader-No-308")
	}
}
