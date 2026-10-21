package ascfake_test

import (
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"testing"

	"github.com/nikeee/store-cli/internal/testutil/ascfake"
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

func send(t *testing.T, method, url string, header map[string]string) (int, string) {
	t.Helper()
	req, err := http.NewRequest(method, url, strings.NewReader("body"))
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
	defer func() { _ = resp.Body.Close() }()
	b, _ := io.ReadAll(resp.Body)
	return resp.StatusCode, string(b)
}

func TestServer(t *testing.T) {
	rec := &recorder{TB: t}
	srv := ascfake.New(rec)
	auth := map[string]string{"Authorization": "Bearer x"}
	srv.Handle("GET", "/v1/apps/{id}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "generic "+r.PathValue("id"))
	})
	srv.Handle("GET", "/v1/apps/{id}", func(w http.ResponseWriter, r *http.Request) {
		_, _ = io.WriteString(w, "latest "+r.PathValue("id"))
	})
	srv.Handle("GET", "/v1/next", ascfake.JSON(http.StatusOK, "apps_page1.json"))

	if code, body := send(t, "GET", srv.URL+"/v1/apps/42", auth); code != 200 || body != "latest 42" {
		t.Errorf("placeholder route: %d %q", code, body)
	}
	if _, body := send(t, "GET", srv.URL+"/v1/next", auth); !strings.Contains(body, `"next": "`+srv.URL+"/v1/apps?cursor=Mg") {
		t.Errorf("{{baseURL}} not replaced: %s", body)
	}
	if code, _ := send(t, "PUT", srv.UploadURL("part1"), nil); code != 200 || len(rec.errs) != 0 {
		t.Errorf("upload: %d %v", code, rec.errs)
	}
	reqs := srv.Requests()
	if last := reqs[len(reqs)-1]; last.Method != "PUT" || last.Path != "/upload/part1" || string(last.Body) != "body" ||
		last.Query.Get("X-Amz-Signature") == "" {
		t.Errorf("recorded %+v", last)
	}
	srv.Handle("PUT", "/upload/denied", func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(http.StatusForbidden) })
	if code, _ := send(t, "PUT", srv.UploadURL("denied"), nil); code != http.StatusForbidden || len(rec.errs) != 0 {
		t.Errorf("registered upload route: %d %v", code, rec.errs)
	}

	tests := []struct {
		name, method, url string
		header            map[string]string
		code              int
		failure           string
	}{
		{"unregistered", "POST", srv.URL + "/v1/apps", auth, http.StatusNotImplemented,
			"no handler for POST /v1/apps; registered: GET /v1/apps/{id}, GET /v1/apps/{id}, GET /v1/next, PUT /upload/denied"},
		{"no token", "GET", srv.URL + "/v1/apps/42", nil, http.StatusUnauthorized, "GET /v1/apps/42 without bearer token"},
		{"upload with token", "PUT", srv.UploadURL("part2"), auth, http.StatusOK, "upload PUT /upload/part2 carries an Authorization header"},
		{"registered upload with token", "PUT", srv.UploadURL("denied"), auth, http.StatusForbidden, "upload PUT /upload/denied carries an Authorization header"},
	}
	for _, tt := range tests {
		rec.errs = nil
		code, _ := send(t, tt.method, tt.url, tt.header)
		if code != tt.code || len(rec.errs) != 1 || !strings.Contains(rec.errs[0], tt.failure) {
			t.Errorf("%s: code=%d failures=%q", tt.name, code, rec.errs)
		}
	}
}
