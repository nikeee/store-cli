package httpx

import (
	"errors"
	"net/url"
	"strings"
	"testing"
)

func TestRedactedURL(t *testing.T) {
	tests := []struct{ in, want string }{
		{"https://a.example/v1/b", "https://a.example/v1/b"},
		{"https://a.example/v1/b?limit=5&sort=-name&include=app", "https://a.example/v1/b?limit=5&sort=-name&include=app"},
		{"https://a.example/u?uploadType=media&access_token=t&Signature=s", "https://a.example/u?uploadType=media&access_token=***&Signature=***"},
		{"https://a.example/u?fields%5Bbuilds%5D=x&exists[k]=true&filter[a]=1", "https://a.example/u?fields%5Bbuilds%5D=x&exists[k]=true&filter[a]=1"},
		{"https://user:pw@a.example/u?flag#frag", "https://a.example/u?flag"},
	}
	for _, tt := range tests {
		u, err := url.Parse(tt.in)
		if err != nil {
			t.Fatal(err)
		}
		if got := redactedURL(u); got != tt.want {
			t.Errorf("redactedURL(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestRedactError(t *testing.T) {
	inner := errors.New("connection refused")
	got := RedactError(&url.Error{Op: "Put", URL: "https://up.example/p?uploadType=media&X-Amz-Signature=sig", Err: inner})
	if want := `Put "https://up.example/p?uploadType=media&X-Amz-Signature=***": connection refused`; got.Error() != want {
		t.Errorf("got %q, want %q", got, want)
	}
	if !errors.Is(got, inner) {
		t.Error("cause lost")
	}
	if got := RedactError(&url.Error{Op: "parse", URL: "https://up.example/%zz?sig=s", Err: inner}); strings.Contains(got.Error(), "sig=s") {
		t.Errorf("unparsable URL kept: %q", got)
	}
	if RedactError(nil) != nil || RedactError(inner).Error() != inner.Error() {
		t.Error("errors without a URL must pass unchanged")
	}
}
