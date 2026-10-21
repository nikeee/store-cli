// Package fakeroute is the routing, request recording and fixture loading
// shared by the fake API servers in internal/testutil.
package fakeroute

import (
	"bytes"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"
	"sync"
)

// HandlerFunc answers one request. Path placeholders are available through
// r.PathValue, e.g. r.PathValue("id") for "/v1/apps/{id}".
type HandlerFunc func(w http.ResponseWriter, r *http.Request)

// Request is a request a fake server received.
type Request struct {
	Method string
	Path   string
	Query  url.Values
	Body   []byte
	Header http.Header
}

// Router matches requests to registered routes and records requests. The
// zero value is ready to use; it is safe for concurrent use.
type Router struct {
	mu       sync.Mutex
	routes   []route
	requests []Request
}

type route struct {
	method string
	path   string
	parts  []segment
	h      HandlerFunc
}

// segment is one path segment of a route: a literal, or a placeholder
// "{name}" followed by an optional literal suffix, as in "{editId}:commit".
type segment struct {
	literal string
	name    string // placeholder name; "" for a literal
	suffix  string
}

func parseSegments(path string) []segment {
	parts := splitPath(path)
	segs := make([]segment, len(parts))
	for i, p := range parts {
		if rest, ok := strings.CutPrefix(p, "{"); ok {
			if name, suffix, ok := strings.Cut(rest, "}"); ok && name != "" {
				segs[i] = segment{name: name, suffix: suffix}
				continue
			}
		}
		segs[i] = segment{literal: p}
	}
	return segs
}

// match reports whether part matches s and returns the placeholder's value.
func (s segment) match(part string) (string, bool) {
	if s.name == "" {
		return "", part == s.literal
	}
	return strings.CutSuffix(part, s.suffix)
}

// Handle registers h for method and path. A path segment "{name}" matches
// any one segment; "{name}:suffix" matches a segment ending in ":suffix" and
// binds the text before it. When several routes match, the last registered
// wins.
func (rt *Router) Handle(method, path string, h HandlerFunc) {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.routes = append(rt.routes, route{method: method, path: path, parts: parseSegments(path), h: h})
}

// Requests returns every recorded request, in order.
func (rt *Router) Requests() []Request {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	return slices.Clone(rt.requests)
}

// Record reads r's body, records r and puts the body back for the handler.
// It records r even when reading the body fails.
func (rt *Router) Record(r *http.Request) ([]byte, error) {
	body, err := io.ReadAll(r.Body)
	r.Body = io.NopCloser(bytes.NewReader(body))
	rt.mu.Lock()
	defer rt.mu.Unlock()
	rt.requests = append(rt.requests, Request{
		Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(), Body: body, Header: r.Header.Clone(),
	})
	return body, err
}

// Match returns the handler of the last registered route matching r and sets
// r's path values, or nil when no route matches.
func (rt *Router) Match(r *http.Request) HandlerFunc {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	parts := splitPath(r.URL.Path)
	for i := len(rt.routes) - 1; i >= 0; i-- {
		ro := rt.routes[i]
		if ro.method != r.Method || len(ro.parts) != len(parts) {
			continue
		}
		values := map[string]string{}
		ok := true
		for j, seg := range ro.parts {
			var v string
			if v, ok = seg.match(parts[j]); !ok {
				break
			}
			if seg.name != "" {
				values[seg.name] = v
			}
		}
		if ok {
			for k, v := range values {
				r.SetPathValue(k, v)
			}
			return ro.h
		}
	}
	return nil
}

// Registered lists the routes for error messages.
func (rt *Router) Registered() string {
	rt.mu.Lock()
	defer rt.mu.Unlock()
	if len(rt.routes) == 0 {
		return "(none)"
	}
	list := make([]string, len(rt.routes))
	for i, ro := range rt.routes {
		list[i] = ro.method + " " + ro.path
	}
	return strings.Join(list, ", ")
}

func splitPath(p string) []string {
	return strings.Split(strings.Trim(p, "/"), "/")
}
