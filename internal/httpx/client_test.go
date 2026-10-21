package httpx_test

import (
	"bytes"
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/nikeee/store-cli/internal/httpx"
)

type sleeper struct{ got []time.Duration }

func (s *sleeper) Sleep(_ context.Context, d time.Duration) error {
	s.got = append(s.got, d)
	return nil
}

func newClient(s *sleeper, log io.Writer) *http.Client {
	return httpx.NewClient(httpx.Options{Verbose: log != nil, Log: log, Sleep: s.Sleep})
}

// seqServer answers with the given statuses in order, then repeats the last one.
func seqServer(t *testing.T, statuses []int, hdr http.Header, bodies *[]string) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	var n atomic.Int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		i := int(n.Add(1)) - 1
		if bodies != nil {
			b, _ := io.ReadAll(r.Body)
			*bodies = append(*bodies, string(b))
		}
		st := statuses[min(i, len(statuses)-1)]
		if st != http.StatusOK {
			for k, v := range hdr {
				w.Header()[k] = v
			}
		}
		w.WriteHeader(st)
	}))
	t.Cleanup(srv.Close)
	return srv, &n
}

func do(t *testing.T, c *http.Client, method, url string, body io.Reader) *http.Response {
	t.Helper()
	req, err := http.NewRequest(method, url, body)
	if err != nil {
		t.Fatal(err)
	}
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = resp.Body.Close() })
	return resp
}

func TestRetryOn429WithRetryAfter(t *testing.T) {
	srv, n := seqServer(t, []int{429, 200}, http.Header{"Retry-After": {"2"}}, nil)
	s := &sleeper{}
	resp := do(t, newClient(s, nil), "GET", srv.URL, nil)
	if resp.StatusCode != 200 || n.Load() != 2 {
		t.Fatalf("status=%d requests=%d", resp.StatusCode, n.Load())
	}
	if len(s.got) != 1 || s.got[0] != 2*time.Second {
		t.Fatalf("sleeps=%v", s.got)
	}
}

func TestRetryOn503ThenSuccess(t *testing.T) {
	srv, _ := seqServer(t, []int{503, 503, 200}, nil, nil)
	s := &sleeper{}
	resp := do(t, newClient(s, nil), "GET", srv.URL, nil)
	if resp.StatusCode != 200 {
		t.Fatalf("status=%d", resp.StatusCode)
	}
	if len(s.got) != 2 {
		t.Fatalf("sleeps=%v", s.got)
	}
	for i, base := range []time.Duration{time.Second, 2 * time.Second} {
		if s.got[i] < base || s.got[i] > base*12/10 {
			t.Errorf("sleep %d = %v, want in [%v, %v]", i, s.got[i], base, base*12/10)
		}
	}
}

func TestBackoffCapsAt30s(t *testing.T) {
	srv, _ := seqServer(t, []int{503}, nil, nil)
	s := &sleeper{}
	c := httpx.NewClient(httpx.Options{MaxAttempts: 8, Sleep: s.Sleep})
	do(t, c, "GET", srv.URL, nil)
	if len(s.got) != 7 {
		t.Fatalf("sleeps=%v", s.got)
	}
	last := s.got[len(s.got)-1]
	if last < 30*time.Second || last > 36*time.Second {
		t.Fatalf("last sleep %v", last)
	}
}

func TestRetryAfterCappedAt60s(t *testing.T) {
	srv, _ := seqServer(t, []int{429, 200}, http.Header{"Retry-After": {"3600"}}, nil)
	s := &sleeper{}
	do(t, newClient(s, nil), "GET", srv.URL, nil)
	if len(s.got) != 1 || s.got[0] != 60*time.Second {
		t.Fatalf("sleeps=%v", s.got)
	}
}

func TestRetryAfterHTTPDate(t *testing.T) {
	date := time.Now().Add(10 * time.Second).UTC().Format(http.TimeFormat)
	srv, _ := seqServer(t, []int{429, 200}, http.Header{"Retry-After": {date}}, nil)
	s := &sleeper{}
	do(t, newClient(s, nil), "GET", srv.URL, nil)
	if len(s.got) != 1 || s.got[0] < 7*time.Second || s.got[0] > 10*time.Second {
		t.Fatalf("sleeps=%v", s.got)
	}
}

func TestNoRetryPostOn500(t *testing.T) {
	srv, n := seqServer(t, []int{500, 200}, nil, nil)
	s := &sleeper{}
	resp := do(t, newClient(s, nil), "POST", srv.URL, strings.NewReader("x"))
	if resp.StatusCode != 500 || n.Load() != 1 || len(s.got) != 0 {
		t.Fatalf("status=%d requests=%d sleeps=%v", resp.StatusCode, n.Load(), s.got)
	}
}

func TestRetryPostOn429(t *testing.T) {
	srv, n := seqServer(t, []int{429, 200}, nil, nil)
	resp := do(t, newClient(&sleeper{}, nil), "POST", srv.URL, strings.NewReader("x"))
	if resp.StatusCode != 200 || n.Load() != 2 {
		t.Fatalf("status=%d requests=%d", resp.StatusCode, n.Load())
	}
}

func TestGivesUpAfterMaxAttempts(t *testing.T) {
	srv, n := seqServer(t, []int{503}, nil, nil)
	resp := do(t, newClient(&sleeper{}, nil), "GET", srv.URL, nil)
	if resp.StatusCode != 503 || n.Load() != 5 {
		t.Fatalf("status=%d requests=%d", resp.StatusCode, n.Load())
	}
}

func TestRetryReplaysBody(t *testing.T) {
	var bodies []string
	srv, _ := seqServer(t, []int{503, 200}, nil, &bodies)
	resp := do(t, newClient(&sleeper{}, nil), "PUT", srv.URL, bytes.NewReader([]byte("payload")))
	if resp.StatusCode != 200 || len(bodies) != 2 || bodies[0] != "payload" || bodies[1] != "payload" {
		t.Fatalf("status=%d bodies=%q", resp.StatusCode, bodies)
	}
}

func TestNoRetryWithoutReplayableBody(t *testing.T) {
	srv, n := seqServer(t, []int{503, 200}, nil, nil)
	body := io.NopCloser(strings.NewReader("x")) // no GetBody
	resp := do(t, newClient(&sleeper{}, nil), "PUT", srv.URL, body)
	if resp.StatusCode != 503 || n.Load() != 1 {
		t.Fatalf("status=%d requests=%d", resp.StatusCode, n.Load())
	}
}

func TestRetryOnTransportError(t *testing.T) {
	srv, _ := seqServer(t, []int{200}, nil, nil)
	url := srv.URL
	srv.Close()
	s := &sleeper{}
	c := newClient(s, nil)
	req, _ := http.NewRequest("GET", url, nil)
	if resp, err := c.Do(req); err == nil {
		_ = resp.Body.Close()
		t.Fatal("expected error")
	}
	if len(s.got) != 4 {
		t.Fatalf("sleeps=%v", s.got)
	}
}

func TestSleepErrorStopsRetries(t *testing.T) {
	srv, n := seqServer(t, []int{503}, nil, nil)
	c := httpx.NewClient(httpx.Options{Sleep: func(context.Context, time.Duration) error { return context.Canceled }})
	req, _ := http.NewRequest("GET", srv.URL, nil)
	if _, err := c.Do(req); err == nil || n.Load() != 1 {
		t.Fatalf("err=%v requests=%d", err, n.Load())
	}
}

func TestVerboseLogFormat(t *testing.T) {
	srv, _ := seqServer(t, []int{503, 200}, nil, nil)
	var log bytes.Buffer
	do(t, newClient(&sleeper{}, &log), "GET", srv.URL+"/v1/builds?limit=5", nil)
	out := log.String()
	for _, want := range []string{"--> GET " + srv.URL + "/v1/builds?limit=5", "<-- 503 GET /v1/builds ", "retry 2/5 after ", " (503)", "<-- 200 GET /v1/builds "} {
		if !strings.Contains(out, want) {
			t.Errorf("log lacks %q:\n%s", want, out)
		}
	}
}

func TestVerboseLogRedactsQuery(t *testing.T) {
	srv, _ := seqServer(t, []int{200}, nil, nil)
	var log bytes.Buffer
	do(t, newClient(&sleeper{}, &log), "GET", srv.URL+"/x?filter[app]=1&X-Amz-Signature=abc&upload_id=xyz", nil)
	out := log.String()
	for _, want := range []string{"filter[app]=1", "X-Amz-Signature=***", "upload_id=***"} {
		if !strings.Contains(out, want) {
			t.Errorf("log lacks %q:\n%s", want, out)
		}
	}
	for _, bad := range []string{"abc", "xyz"} {
		if strings.Contains(out, bad) {
			t.Errorf("log leaks %q:\n%s", bad, out)
		}
	}
}

func TestVerboseNeverLogsAuthorization(t *testing.T) {
	srv, _ := seqServer(t, []int{200}, nil, nil)
	var log bytes.Buffer
	c := newClient(&sleeper{}, &log)
	req, _ := http.NewRequest("GET", srv.URL, strings.NewReader("body-secret"))
	req.Header.Set("Authorization", "Bearer secret")
	resp, err := c.Do(req)
	if err != nil {
		t.Fatal(err)
	}
	_ = resp.Body.Close()
	if strings.Contains(log.String(), "secret") {
		t.Fatalf("log leaks secret:\n%s", log.String())
	}
}

func TestQuietWithoutVerbose(t *testing.T) {
	srv, _ := seqServer(t, []int{503, 200}, nil, nil)
	var log bytes.Buffer
	c := httpx.NewClient(httpx.Options{Log: &log, Sleep: (&sleeper{}).Sleep})
	do(t, c, "GET", srv.URL, nil)
	if log.Len() != 0 {
		t.Fatalf("log=%q", log.String())
	}
}
