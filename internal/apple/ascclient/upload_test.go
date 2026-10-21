package ascclient_test

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/nikeee/store-cli/internal/apple/ascclient"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
)

func TestUploadPartsSendsNoAuthorization(t *testing.T) {
	srv := ascfake.New(t)
	c := newClient(t, srv, time.Now)
	hdr := map[string]string{"Content-Type": "application/octet-stream"}
	ops := []ascclient.UploadOperation{
		{Method: "PUT", URL: srv.UploadURL("part1"), Offset: 0, Length: 5, Headers: hdr},
		{Method: "PUT", URL: srv.UploadURL("part2"), Offset: 5, Length: 3, Headers: hdr},
	}
	if err := c.UploadParts(t.Context(), ops, strings.NewReader("helloabc")); err != nil {
		t.Fatal(err)
	}

	got := map[string]string{}
	for _, r := range srv.Requests() {
		got[r.Method+" "+r.Path] = string(r.Body)
		if a := r.Header.Get("Authorization"); a != "" {
			t.Errorf("%s carries Authorization %q", r.Path, a)
		}
		if ct := r.Header.Get("Content-Type"); ct != "application/octet-stream" {
			t.Errorf("%s Content-Type = %q", r.Path, ct)
		}
	}
	want := map[string]string{"PUT /upload/part1": "hello", "PUT /upload/part2": "abc"}
	if len(got) != len(want) || got["PUT /upload/part1"] != "hello" || got["PUT /upload/part2"] != "abc" {
		t.Errorf("uploads = %v, want %v", got, want)
	}
}

// Presigned URLs carry their signature in the query; a failing upload must
// not print it.
func TestUploadPartsErrorHidesSignature(t *testing.T) {
	const secret = "X-Amz-Signature=topsecret"
	forbidden := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	}))
	t.Cleanup(forbidden.Close)
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()

	for name, base := range map[string]string{"status 403": forbidden.URL, "connection refused": closed.URL} {
		t.Run(name, func(t *testing.T) {
			c := newClient(t, ascfake.New(t), time.Now)
			op := ascclient.UploadOperation{Method: "PUT", URL: base + "/upload/part1?" + secret, Length: 5}
			err := c.UploadParts(t.Context(), []ascclient.UploadOperation{op}, strings.NewReader("hello"))
			if err == nil || strings.Contains(err.Error(), "topsecret") {
				t.Fatalf("err = %v", err)
			}
		})
	}
}
