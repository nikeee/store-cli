// Package ascfake is a programmable fake App Store Connect server for tests.
package ascfake

import (
	"bytes"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strings"
	"sync"
	"testing"
)

// HandlerFunc answers one request. Path placeholders are available through
// r.PathValue, e.g. r.PathValue("id") for "/v1/apps/{id}".
type HandlerFunc func(w http.ResponseWriter, r *http.Request)

// Request is a request the server received.
type Request struct {
	Method string
	Path   string
	Query  url.Values
	Body   []byte
	Header http.Header
}

type route struct {
	method string
	path   string
	parts  []string
	h      HandlerFunc
}

// Server is a fake App Store Connect API. API requests must carry a bearer
// token; requests to UploadURL must not carry an Authorization header.
type Server struct {
	URL string

	t        testing.TB
	mu       sync.Mutex
	routes   []route
	requests []Request
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

// Handle registers h for method and path. A path segment "{name}" matches
// any one segment. When several routes match, the last registered wins.
func (s *Server) Handle(method, path string, h HandlerFunc) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.routes = append(s.routes, route{method: method, path: path, parts: splitPath(path), h: h})
}

// Requests returns every request received so far, in order.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.requests)
}

// UploadURL returns a presigned-style upload URL served by the fake. The
// query carries a fake signature that must never show up in any output.
// Uploads answer 200 unless a route for their path is registered, e.g.
// Handle("PUT", "/upload/{name}", h) to fail or delay them.
func (s *Server) UploadURL(name string) string {
	return s.URL + uploadPrefix + name + "?X-Amz-Signature=fake-signature-" + name
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.t.Errorf("ascfake: read body of %s %s: %v", r.Method, r.URL.Path, err)
	}
	r.Body = io.NopCloser(bytes.NewReader(body))

	s.mu.Lock()
	s.requests = append(s.requests, Request{
		Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(), Body: body, Header: r.Header.Clone(),
	})
	h := s.match(r)
	s.mu.Unlock()

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
		s.t.Errorf("ascfake: no handler for %s %s; registered: %s", r.Method, r.URL.Path, s.registered())
		w.WriteHeader(http.StatusNotImplemented)
		return
	}
	h(w, r)
}

// match returns the handler of the last registered route matching r and sets
// r's path values. The caller holds s.mu.
func (s *Server) match(r *http.Request) HandlerFunc {
	parts := splitPath(r.URL.Path)
	for i := len(s.routes) - 1; i >= 0; i-- {
		rt := s.routes[i]
		if rt.method != r.Method || len(rt.parts) != len(parts) {
			continue
		}
		values := map[string]string{}
		ok := true
		for j, p := range rt.parts {
			if name, isVar := strings.CutPrefix(p, "{"); isVar {
				values[strings.TrimSuffix(name, "}")] = parts[j]
			} else if p != parts[j] {
				ok = false
				break
			}
		}
		if ok {
			for k, v := range values {
				r.SetPathValue(k, v)
			}
			return rt.h
		}
	}
	return nil
}

// registered lists the routes for error messages.
func (s *Server) registered() string {
	s.mu.Lock()
	defer s.mu.Unlock()
	if len(s.routes) == 0 {
		return "(none)"
	}
	list := make([]string, len(s.routes))
	for i, rt := range s.routes {
		list[i] = rt.method + " " + rt.path
	}
	return strings.Join(list, ", ")
}

func splitPath(p string) []string {
	return strings.Split(strings.Trim(p, "/"), "/")
}
