package ascclient_test

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"math/big"
	"net/http"
	"strings"
	"testing"
	"time"

	"github.com/nikeee/store-cli/internal/apple/asc"
	"github.com/nikeee/store-cli/internal/apple/ascclient"
	"github.com/nikeee/store-cli/internal/testutil/ascfake"
)

// fetchToken sends one API request and returns the bearer token the fake saw.
func fetchToken(t *testing.T, srv *ascfake.Server, c *ascclient.Client) string {
	t.Helper()
	if _, err := c.AppsGetCollection(t.Context(), asc.AppsGetCollectionParams{Limit: 1}); err != nil {
		t.Fatal(err)
	}
	reqs := srv.Requests()
	tok, ok := strings.CutPrefix(reqs[len(reqs)-1].Header.Get("Authorization"), "Bearer ")
	if !ok {
		t.Fatalf("no bearer token: %v", reqs[len(reqs)-1].Header)
	}
	return tok
}

func TestTokenClaims(t *testing.T) {
	now := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	srv := ascfake.New(t)
	srv.Handle("GET", "/v1/apps", ascfake.JSON(http.StatusOK, "apps_one.json"))
	tok := fetchToken(t, srv, newClient(t, srv, func() time.Time { return now }))

	parts := strings.Split(tok, ".")
	if len(parts) != 3 || strings.Contains(tok, "=") {
		t.Fatalf("token is not three unpadded base64url parts: %q", tok)
	}
	decode := func(s string) []byte {
		b, err := base64.RawURLEncoding.DecodeString(s)
		if err != nil {
			t.Fatal(err)
		}
		return b
	}
	if h := string(decode(parts[0])); h != `{"alg":"ES256","kid":"`+ascfake.KeyID+`","typ":"JWT"}` {
		t.Errorf("header = %s", h)
	}

	var claims map[string]any
	if err := json.Unmarshal(decode(parts[1]), &claims); err != nil {
		t.Fatal(err)
	}
	iat, _ := claims["iat"].(float64)
	exp, _ := claims["exp"].(float64)
	if len(claims) != 4 || claims["iss"] != ascfake.IssuerID || claims["aud"] != "appstoreconnect-v1" ||
		int64(iat) != now.Unix() || time.Duration(exp-iat)*time.Second != 19*time.Minute {
		t.Errorf("claims = %v", claims)
	}

	sig := decode(parts[2])
	if len(sig) != 64 {
		t.Fatalf("signature has %d bytes, want 64 (r||s)", len(sig))
	}
	block, _ := pem.Decode(ascfake.KeyPEM)
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		t.Fatal(err)
	}
	digest := sha256.Sum256([]byte(parts[0] + "." + parts[1]))
	r, s := new(big.Int).SetBytes(sig[:32]), new(big.Int).SetBytes(sig[32:])
	if !ecdsa.Verify(&key.(*ecdsa.PrivateKey).PublicKey, digest[:], r, s) {
		t.Error("ES256 signature does not verify with the test key")
	}
}

func TestTokenCachedAndRenewed(t *testing.T) {
	t0 := time.Date(2026, 10, 21, 12, 0, 0, 0, time.UTC)
	tests := []struct {
		name    string
		advance time.Duration
		renewed bool
	}{
		{"within 10 minutes", 10 * time.Minute, false},
		{"60s left", 18 * time.Minute, false},
		{"less than 60s left", 18*time.Minute + time.Second, true},
		{"19 minutes", 19 * time.Minute, true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			now := t0
			srv := ascfake.New(t)
			srv.Handle("GET", "/v1/apps", ascfake.JSON(http.StatusOK, "apps_one.json"))
			c := newClient(t, srv, func() time.Time { return now })
			first := fetchToken(t, srv, c)
			now = t0.Add(tt.advance)
			if renewed := fetchToken(t, srv, c) != first; renewed != tt.renewed {
				t.Errorf("renewed = %v, want %v", renewed, tt.renewed)
			}
		})
	}
}
