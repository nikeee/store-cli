// Package playfake is a programmable fake Google Play Developer API server
// for tests, with an OAuth token endpoint and Google's upload protocols.
package playfake

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"net/url"
	"slices"
	"strconv"
	"strings"
	"sync"
	"testing"
)

// HandlerFunc answers one request. Path placeholders are available through
// r.PathValue, e.g. r.PathValue("editId") for ".../edits/{editId}".
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

// Server is a fake Google Play Developer API. POST /token issues
// AccessToken; every other request must carry it as bearer token.
//
// Requests to /upload/... are uploads: uploadType=media and
// uploadType=multipart are answered at once, uploadType=resumable starts a
// session whose Location (the same URL plus upload_id) takes the chunks by
// PUT or POST with Content-Range. Incomplete chunks get 308 (or 200 with
// X-HTTP-Status-Code-Override: 308 when the request sets X-GUploader-No-308).
// When the media is complete, the route registered for the upload's method
// and path answers, with the media as request body.
type Server struct {
	URL string

	t        testing.TB
	mu       sync.Mutex
	routes   []route
	requests []Request
	sessions map[string]*session
	nextID   int
}

// session is a resumable upload in progress.
type session struct {
	method string
	media  []byte
}

// AccessToken is the token the fake issues and requires.
const AccessToken = "fake-access-token"

const (
	tokenPath    = "/token"
	uploadPrefix = "/upload/"
	jwtBearer    = "urn:ietf:params:oauth:grant-type:jwt-bearer"
	playScope    = "https://www.googleapis.com/auth/androidpublisher"
)

// New starts a server that is closed when the test ends.
func New(t testing.TB) *Server {
	t.Helper()
	s := &Server{t: t, sessions: map[string]*session{}}
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

// Requests returns every API and upload request received so far, in order.
// Token requests are not included.
func (s *Server) Requests() []Request {
	s.mu.Lock()
	defer s.mu.Unlock()
	return slices.Clone(s.requests)
}

func (s *Server) serve(w http.ResponseWriter, r *http.Request) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		s.t.Errorf("playfake: read body of %s %s: %v", r.Method, r.URL.Path, err)
	}
	r.Body = io.NopCloser(bytes.NewReader(body))
	if r.Method == http.MethodPost && r.URL.Path == tokenPath {
		s.token(w, r)
		return
	}

	s.mu.Lock()
	s.requests = append(s.requests, Request{
		Method: r.Method, Path: r.URL.Path, Query: r.URL.Query(), Body: body, Header: r.Header.Clone(),
	})
	s.mu.Unlock()

	if r.Header.Get("Authorization") != "Bearer "+AccessToken {
		s.t.Errorf("playfake: %s %s without the fake access token", r.Method, r.URL.Path)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}
	if strings.HasPrefix(r.URL.Path, uploadPrefix) {
		s.upload(w, r, body)
		return
	}
	s.answer(w, r)
}

// token answers the JWT bearer grant of a service account with AccessToken.
func (s *Server) token(w http.ResponseWriter, r *http.Request) {
	grant, assertion := r.PostFormValue("grant_type"), r.PostFormValue("assertion")
	var claims struct {
		Scope string `json:"scope"`
	}
	if parts := strings.Split(assertion, "."); len(parts) == 3 {
		if b, err := base64.RawURLEncoding.DecodeString(parts[1]); err == nil {
			_ = json.Unmarshal(b, &claims)
		}
	}
	if grant != jwtBearer || claims.Scope != playScope {
		s.t.Errorf("playfake: token request with grant_type %q and scope %q", grant, claims.Scope)
		http.Error(w, `{"error":"invalid_grant"}`, http.StatusBadRequest)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = fmt.Fprintf(w, `{"access_token":%q,"token_type":"Bearer","expires_in":3600}`, AccessToken)
}

// upload handles the upload protocols, see Server.
func (s *Server) upload(w http.ResponseWriter, r *http.Request, body []byte) {
	q := r.URL.Query()
	if id := q.Get("upload_id"); id != "" {
		s.chunk(w, r, id, body)
		return
	}
	switch t := q.Get("uploadType"); t {
	case "media":
		s.complete(w, r, r.Method, body)
	case "multipart":
		media, err := lastPart(r.Header.Get("Content-Type"), body)
		if err != nil {
			s.t.Errorf("playfake: multipart upload %s %s: %v", r.Method, r.URL.Path, err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		s.complete(w, r, r.Method, media)
	case "resumable":
		s.mu.Lock()
		s.nextID++
		id := "fake-upload-id-" + strconv.Itoa(s.nextID)
		s.sessions[id] = &session{method: r.Method}
		s.mu.Unlock()
		q.Set("upload_id", id)
		w.Header().Set("Location", s.URL+r.URL.Path+"?"+q.Encode())
		w.WriteHeader(http.StatusOK)
	default:
		s.t.Errorf("playfake: upload %s %s with uploadType %q", r.Method, r.URL.Path, t)
		w.WriteHeader(http.StatusBadRequest)
	}
}

// chunk takes one chunk of a resumable upload.
func (s *Server) chunk(w http.ResponseWriter, r *http.Request, id string, body []byte) {
	sess, done, status, err := s.appendChunk(id, r.Header.Get("Content-Range"), body)
	if err != nil {
		s.t.Errorf("playfake: upload %s %s: %v", r.Method, r.URL.Path, err)
		w.WriteHeader(status)
		return
	}
	if done {
		s.complete(w, r, sess.method, sess.media)
		return
	}
	if len(sess.media) > 0 {
		w.Header().Set("Range", fmt.Sprintf("bytes=0-%d", len(sess.media)-1))
	}
	if r.Header.Get("X-GUploader-No-308") == "yes" {
		w.Header().Set("X-HTTP-Status-Code-Override", "308")
		w.WriteHeader(http.StatusOK)
		return
	}
	w.WriteHeader(http.StatusPermanentRedirect)
}

// appendChunk appends body to session id and reports whether the upload is
// complete; a complete session is removed. On error, status is the HTTP
// status to answer with.
func (s *Server) appendChunk(id, contentRange string, body []byte) (sess session, done bool, status int, err error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	sp := s.sessions[id]
	if sp == nil {
		return session{}, false, http.StatusNotFound, fmt.Errorf("unknown upload session %q", id)
	}
	start, total, err := parseContentRange(contentRange)
	if err == nil && start >= 0 && start != int64(len(sp.media)) {
		err = fmt.Errorf("chunk starts at %d, have %d bytes", start, len(sp.media))
	}
	if err != nil {
		return session{}, false, http.StatusBadRequest, err
	}
	sp.media = append(sp.media, body...)
	done = total >= 0 && int64(len(sp.media)) == total
	if done {
		delete(s.sessions, id)
	}
	return *sp, done, 0, nil
}

// complete hands a finished upload to the route for method and r's path.
func (s *Server) complete(w http.ResponseWriter, r *http.Request, method string, media []byte) {
	r = r.Clone(r.Context())
	r.Method = method
	r.Body = io.NopCloser(bytes.NewReader(media))
	r.ContentLength = int64(len(media))
	s.answer(w, r)
}

// answer runs the route matching r.
func (s *Server) answer(w http.ResponseWriter, r *http.Request) {
	s.mu.Lock()
	h := s.match(r)
	s.mu.Unlock()
	if h == nil {
		s.t.Errorf("playfake: no handler for %s %s; registered: %s", r.Method, r.URL.Path, s.registered())
		w.WriteHeader(http.StatusNotImplemented)
		return
	}
	h(w, r)
}

// parseContentRange parses "bytes <first>-<last>/<total>", where total may
// be "*" (unknown, -1), and "bytes */<total>" (no data, start -1).
func parseContentRange(v string) (start, total int64, err error) {
	rng, size, ok := strings.Cut(strings.TrimPrefix(v, "bytes "), "/")
	if !ok || !strings.HasPrefix(v, "bytes ") {
		return 0, 0, fmt.Errorf("bad Content-Range %q", v)
	}
	total = -1
	if size != "*" {
		if total, err = strconv.ParseInt(size, 10, 64); err != nil {
			return 0, 0, fmt.Errorf("bad Content-Range %q", v)
		}
	}
	if rng == "*" {
		return -1, total, nil
	}
	first, _, _ := strings.Cut(rng, "-")
	if start, err = strconv.ParseInt(first, 10, 64); err != nil {
		return 0, 0, fmt.Errorf("bad Content-Range %q", v)
	}
	return start, total, nil
}

// lastPart returns the last part of a multipart/related body: the media
// after the JSON metadata.
func lastPart(contentType string, body []byte) ([]byte, error) {
	_, params, err := mime.ParseMediaType(contentType)
	if err != nil {
		return nil, err
	}
	mr := multipart.NewReader(bytes.NewReader(body), params["boundary"])
	var last []byte
	for {
		p, err := mr.NextPart()
		if errors.Is(err, io.EOF) {
			return last, nil
		}
		if err != nil {
			return nil, err
		}
		if last, err = io.ReadAll(p); err != nil {
			return nil, err
		}
	}
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
