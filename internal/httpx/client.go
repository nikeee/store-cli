// Package httpx provides the single HTTP client used for every API call:
// retries with backoff and a redacted --verbose request log.
package httpx

import (
	"context"
	"fmt"
	"io"
	"math/rand/v2"
	"net/http"
	"strconv"
	"time"
)

const (
	defaultMaxAttempts = 5
	maxRetryAfter      = 60 * time.Second
	maxBackoff         = 30 * time.Second
	baseBackoff        = time.Second
	jitterFraction     = 0.2
)

// Options configures NewClient.
type Options struct {
	Verbose     bool
	Log         io.Writer
	MaxAttempts int                                              // default 5
	Sleep       func(ctx context.Context, d time.Duration) error // injectable for tests

	// jitter returns a factor in [0, 1) scaling the 20% jitter; test hook.
	jitter func() float64
}

// NewClient returns an http.Client whose transport retries and logs.
func NewClient(opts Options) *http.Client {
	if opts.MaxAttempts <= 0 {
		opts.MaxAttempts = defaultMaxAttempts
	}
	if opts.Sleep == nil {
		opts.Sleep = sleepCtx
	}
	if opts.jitter == nil {
		opts.jitter = rand.Float64
	}
	return &http.Client{Transport: &transport{opts: opts, base: http.DefaultTransport}}
}

func sleepCtx(ctx context.Context, d time.Duration) error {
	t := time.NewTimer(d)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

type transport struct {
	opts Options
	base http.RoundTripper
}

func (t *transport) logf(format string, args ...any) {
	if t.opts.Verbose && t.opts.Log != nil {
		_, _ = fmt.Fprintf(t.opts.Log, format+"\n", args...)
	}
}

func (t *transport) RoundTrip(req *http.Request) (*http.Response, error) {
	replayable := req.Body == nil || req.Body == http.NoBody || req.GetBody != nil
	target := redactedURL(req.URL)
	t.logf("--> %s %s", req.Method, target)

	for attempt := 1; ; attempt++ {
		r := req
		if attempt > 1 && req.GetBody != nil {
			body, err := req.GetBody()
			if err != nil {
				return nil, err
			}
			r = req.Clone(req.Context())
			r.Body = body
		}
		start := time.Now()
		resp, err := t.base.RoundTrip(r)
		elapsed := time.Since(start).Round(time.Millisecond)
		switch {
		case err != nil:
			t.logf("<-- error %s %s %s: %v", req.Method, req.URL.EscapedPath(), elapsed, err)
		default:
			t.logf("<-- %d %s %s %s", resp.StatusCode, req.Method, req.URL.EscapedPath(), elapsed)
		}

		if attempt >= t.opts.MaxAttempts || !replayable || !retryable(req.Method, resp, err) {
			return resp, err
		}
		if err != nil && req.Context().Err() != nil {
			return resp, err
		}

		wait := t.wait(attempt, resp)
		reason := "transport error"
		if resp != nil {
			reason = strconv.Itoa(resp.StatusCode)
			// Drain so the connection can be reused.
			_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<20))
			_ = resp.Body.Close()
		}
		t.logf("retry %d/%d after %s (%s)", attempt+1, t.opts.MaxAttempts, wait, reason)
		if serr := t.opts.Sleep(req.Context(), wait); serr != nil {
			return nil, serr
		}
	}
}

// retryable applies the method and status rules: POST is only retried on 429.
func retryable(method string, resp *http.Response, err error) bool {
	if err != nil {
		return method != http.MethodPost
	}
	switch resp.StatusCode {
	case http.StatusTooManyRequests:
		return true
	case http.StatusInternalServerError, http.StatusBadGateway,
		http.StatusServiceUnavailable, http.StatusGatewayTimeout:
		return method != http.MethodPost
	}
	return false
}

// wait is Retry-After (capped, no jitter: the server named the delay) if
// present, else exponential backoff plus up to 20% jitter.
func (t *transport) wait(attempt int, resp *http.Response) time.Duration {
	if resp != nil {
		if ra, ok := parseRetryAfter(resp.Header.Get("Retry-After"), time.Now()); ok {
			return min(ra, maxRetryAfter)
		}
	}
	d := baseBackoff
	for i := 1; i < attempt && d < maxBackoff; i++ {
		d *= 2
	}
	d = min(d, maxBackoff)
	return d + time.Duration(float64(d)*jitterFraction*t.opts.jitter())
}

func parseRetryAfter(v string, now time.Time) (time.Duration, bool) {
	if v == "" {
		return 0, false
	}
	if secs, err := strconv.Atoi(v); err == nil {
		if secs < 0 {
			return 0, false
		}
		return time.Duration(secs) * time.Second, true
	}
	if at, err := http.ParseTime(v); err == nil {
		return max(at.Sub(now), 0), true
	}
	return 0, false
}
