package ascclient

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"sync"
	"time"

	"github.com/nikeee/store-cli/internal/config"
)

const (
	tokenLifetime = 19 * time.Minute // Apple allows at most 20
	tokenRenewal  = 60 * time.Second // renew when less than this is left
	tokenAudience = "appstoreconnect-v1"
)

// tokenSource makes the ES256 JWT for the App Store Connect API and caches it
// until less than tokenRenewal of its lifetime is left.
// https://developer.apple.com/documentation/appstoreconnectapi/generating-tokens-for-api-requests
type tokenSource struct {
	issuerID, keyID string
	key             *ecdsa.PrivateKey
	now             func() time.Time

	mu      sync.Mutex
	token   string
	expires time.Time
}

func newTokenSource(creds config.AppleCredentials, now func() time.Time) (*tokenSource, error) {
	key, err := config.ParseAppleKey(creds.PrivateKeyPEM)
	if err != nil {
		return nil, err
	}
	return &tokenSource{issuerID: creds.IssuerID, keyID: creds.KeyID, key: key, now: now}, nil
}

// Token returns the cached token or signs a new one.
func (s *tokenSource) Token() (string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := s.now()
	if s.token != "" && s.expires.Sub(now) >= tokenRenewal {
		return s.token, nil
	}
	exp := now.Add(tokenLifetime)
	tok, err := s.sign(now, exp)
	if err != nil {
		return "", err
	}
	s.token, s.expires = tok, exp
	return tok, nil
}

func (s *tokenSource) sign(iat, exp time.Time) (string, error) {
	header, err := json.Marshal(struct {
		Alg string `json:"alg"`
		Kid string `json:"kid"`
		Typ string `json:"typ"`
	}{"ES256", s.keyID, "JWT"})
	if err != nil {
		return "", err
	}
	claims, err := json.Marshal(struct {
		Iss string `json:"iss"`
		Iat int64  `json:"iat"`
		Exp int64  `json:"exp"`
		Aud string `json:"aud"`
	}{s.issuerID, iat.Unix(), exp.Unix(), tokenAudience})
	if err != nil {
		return "", err
	}
	enc := base64.RawURLEncoding
	input := enc.EncodeToString(header) + "." + enc.EncodeToString(claims)
	digest := sha256.Sum256([]byte(input))
	r, sv, err := ecdsa.Sign(rand.Reader, s.key, digest[:])
	if err != nil {
		return "", err
	}
	// JWS wants the raw r||s form (IEEE P1363), 32 bytes each for P-256, not DER.
	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	sv.FillBytes(sig[32:])
	return input + "." + enc.EncodeToString(sig), nil
}
