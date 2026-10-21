package config

import (
	"bytes"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/x509"
	"encoding/json"
	"encoding/pem"
	"strings"

	"github.com/nikeee/store-cli/internal/clierr"
)

// AppleCredentials are the App Store Connect API key settings.
type AppleCredentials struct {
	IssuerID, KeyID string
	PrivateKeyPEM   []byte
}

// LoadAppleCredentials reads the App Store Connect API key from env. The
// private key variable holds either the PEM content or a path to a .p8 file.
// No error ever contains a value from the environment.
func LoadAppleCredentials(env Env, readFile func(string) ([]byte, error)) (AppleCredentials, error) {
	var missing []string
	get := func(name string) string {
		v, _ := env(name)
		v = strings.TrimSpace(v)
		if v == "" {
			missing = append(missing, name)
		}
		return v
	}
	issuer := get(EnvAppleIssuerID)
	keyID := get(EnvAppleKeyID)
	raw := get(EnvApplePrivateKey)
	if len(missing) > 0 {
		return AppleCredentials{}, clierr.New("missing env vars: %s", strings.Join(missing, ", "))
	}

	raw = trimQuotes(raw)
	var pemBytes []byte
	if strings.Contains(raw, "-----BEGIN") {
		if !strings.Contains(raw, "\n") {
			raw = strings.ReplaceAll(raw, `\n`, "\n")
		}
		pemBytes = []byte(raw)
	} else {
		b, err := readFile(raw)
		if err != nil {
			return AppleCredentials{}, clierr.New("%s is neither PEM content nor a readable file path", EnvApplePrivateKey)
		}
		pemBytes = b
	}

	if err := checkAppleKey(pemBytes); err != nil {
		return AppleCredentials{}, err
	}
	return AppleCredentials{IssuerID: issuer, KeyID: keyID, PrivateKeyPEM: pemBytes}, nil
}

func checkAppleKey(pemBytes []byte) error {
	bad := clierr.New("%s is not a PKCS#8 EC private key (.p8)", EnvApplePrivateKey)
	block, _ := pem.Decode(pemBytes)
	if block == nil {
		return bad
	}
	key, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return bad
	}
	ec, ok := key.(*ecdsa.PrivateKey)
	if !ok || ec.Curve != elliptic.P256() {
		return bad
	}
	return nil
}

// trimQuotes removes one pair of matching surrounding quotes.
func trimQuotes(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && (s[0] == '"' || s[0] == '\'') && s[len(s)-1] == s[0] {
		return strings.TrimSpace(s[1 : len(s)-1])
	}
	return s
}

// LoadGoogleServiceAccount returns the service account JSON from env. The
// variable holds either the JSON content or a path to a JSON file.
func LoadGoogleServiceAccount(env Env, readFile func(string) ([]byte, error)) ([]byte, error) {
	v, _ := env(EnvGoogleServiceAccount)
	v = strings.TrimSpace(v)
	if v == "" {
		return nil, clierr.New("missing env vars: %s", EnvGoogleServiceAccount)
	}
	data := []byte(v)
	if !strings.HasPrefix(v, "{") {
		b, err := readFile(v)
		if err != nil {
			return nil, clierr.New("%s is neither JSON content nor a readable file path", EnvGoogleServiceAccount)
		}
		data = bytes.TrimSpace(b)
	}
	var sa struct {
		Type        string `json:"type"`
		ClientEmail string `json:"client_email"`
		PrivateKey  string `json:"private_key"`
	}
	if err := json.Unmarshal(data, &sa); err != nil {
		return nil, clierr.New("%s is not valid JSON", EnvGoogleServiceAccount)
	}
	if sa.Type != "service_account" || sa.ClientEmail == "" || sa.PrivateKey == "" {
		return nil, clierr.New(`%s is not a service account key (needs "type": "service_account", client_email and private_key)`, EnvGoogleServiceAccount)
	}
	return data, nil
}
