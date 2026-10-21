// Package ascfake is a programmable fake App Store Connect server for tests.
package ascfake

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/testutil/fakeroute"
)

// HandlerFunc answers one request. Path placeholders are available through
// r.PathValue, e.g. r.PathValue("id") for "/v1/apps/{id}".
type HandlerFunc = fakeroute.HandlerFunc

// Request is a request the server received.
type Request = fakeroute.Request

// Server is a fake App Store Connect API. API requests must carry a bearer
// token; requests to UploadURL must not carry an Authorization header.
// Handle registers routes, Requests returns every request received so far.
type Server struct {
	fakeroute.Router

	URL string

	t testing.TB
}

const uploadPrefix = "/upload/"

// New starts a server that is closed when the test ends.
func New(t testing.TB) *Server {
	t.Helper()
	s := &Server{t: t}
	srv := httptest.NewServer(http.HandlerFunc(s.serve))
	t.Cleanup(srv.Close)
	s.URL = srv.URL
	return s
}

// UploadURL returns a presigned-style upload URL served by the fake. The
// query carries a fake signature that must never show up in any output.
// Uploads answer 200 unless a route for their path is registered, e.g.
// Handle("PUT", "/upload/{name}", h) to fail or delay them.
func (s *Server) UploadURL(name string) string {
	return s.URL + uploadPrefix + name + "?X-Amz-Signature=fake-signature-" + name
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	if _, err := s.Record(r); err != nil {
		s.t.Errorf("ascfake: read body of %s %s: %v", r.Method, r.URL.Path, err)
	}
	h := s.Match(r)

	if strings.HasPrefix(r.URL.Path, uploadPrefix) {
		if r.Header.Get("Authorization") != "" {
			s.t.Errorf("ascfake: upload %s %s carries an Authorization header", r.Method, r.URL.Path)
		}
		if h == nil {
			w.WriteHeader(http.StatusOK)
			return
		}
		h(w, r)
		return
	}
	if !strings.HasPrefix(r.Header.Get("Authorization"), "Bearer ") {
		s.t.Errorf("ascfake: %s %s without bearer token", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if h == nil {
		s.t.Errorf("ascfake: no handler for %s %s; registered: %s", r.Method, r.URL.Path, s.Registered())
		w.WriteHeader(http.StatusNotImplemented)
		return
	}
	h(w, r)
}
