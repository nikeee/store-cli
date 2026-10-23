package ascclient_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nikeee/store-cli/internal/apple/ascclient"
	"github.com/nikeee/store-cli/internal/httpx"
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
	srv := ascfake.New(t)
	srv.Handle("PUT", "/upload/{name}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusForbidden)
	})
	// A closed server stands in for an unreachable upload host.
	closed := httptest.NewServer(http.NotFoundHandler())
	closed.Close()

	for name, url := range map[string]string{
		"status 403":         srv.UploadURL("part1"),
		"connection refused": closed.URL + "/upload/part1?X-Amz-Signature=fake-signature-part1",
	} {
		t.Run(name, func(t *testing.T) {
			c := newClient(t, srv, time.Now)
			op := ascclient.UploadOperation{Method: "PUT", URL: url, Length: 5}
			err := c.UploadParts(t.Context(), []ascclient.UploadOperation{op}, strings.NewReader("hello"))
			if err == nil || strings.Contains(err.Error(), "fake-signature") {
				t.Fatalf("err = %v", err)
			}
		})
	}
}

// A part that fails with 503 is sent again by the httpx client, with the
// same bytes.
func TestUploadPartsRetries(t *testing.T) {
	srv := ascfake.New(t)
	var failed atomic.Bool
	srv.Handle("PUT", "/upload/{name}", func(w http.ResponseWriter, _ *http.Request) {
		if !failed.Swap(true) {
			w.WriteHeader(http.StatusServiceUnavailable)
		}
	})
	noSleep := func(context.Context, time.Duration) error { return nil }
	c, err := ascclient.New(ascfake.Credentials(), httpx.NewClient(httpx.Options{Sleep: noSleep}), srv.URL, time.Now)
	if err != nil {
		t.Fatal(err)
	}
	op := ascclient.UploadOperation{Method: "PUT", URL: srv.UploadURL("part2"), Offset: 5, Length: 3}
	if err := c.UploadParts(t.Context(), []ascclient.UploadOperation{op}, strings.NewReader("helloabc")); err != nil {
		t.Fatal(err)
	}
	var bodies []string
	for _, r := range srv.Requests() {
		bodies = append(bodies, string(r.Body))
	}
	if len(bodies) != 2 || bodies[0] != "abc" || bodies[1] != "abc" {
		t.Errorf("bodies = %q", bodies)
	}
}

// At most four parts are in flight at once.
func TestUploadPartsAtMostFourAtOnce(t *testing.T) {
	var inFlight, most atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		n := inFlight.Add(1)
		for {
			m := most.Load()
			if n <= m || most.CompareAndSwap(m, n) {
				break
			}
		}
		time.Sleep(20 * time.Millisecond)
		inFlight.Add(-1)
	}))
	t.Cleanup(srv.Close)
	c := newClient(t, ascfake.New(t), time.Now)
	var ops []ascclient.UploadOperation
	for i := range 10 {
		ops = append(ops, ascclient.UploadOperation{Method: "PUT", URL: srv.URL + "/part", Offset: int64(i), Length: 1})
	}
	if err := c.UploadParts(t.Context(), ops, strings.NewReader("0123456789")); err != nil {
		t.Fatal(err)
	}
	if m := most.Load(); m > 4 || m < 2 {
		t.Errorf("at most %d parts in flight, want 2 to 4", m)
	}
}
