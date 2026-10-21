package httpx

import (
	"net/url"
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
