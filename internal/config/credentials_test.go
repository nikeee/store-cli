package config_test

import (
	"crypto/elliptic"
	"errors"
	"os"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/config"
)

func envOf(m map[string]string) config.Env {
	return func(name string) (string, bool) {
		v, ok := m[name]
		return v, ok
	}
}

func readTestdata(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile("testdata/" + name)
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestLoadAppleCredentialsShapes(t *testing.T) {
	ec := readTestdata(t, "test_ec_p256.p8")
	rsa := readTestdata(t, "test_rsa.p8")
	files := map[string]string{"/keys/AuthKey.p8": ec}
	readFile := func(p string) ([]byte, error) {
		if v, ok := files[p]; ok {
			return []byte(v), nil
		}
		return nil, errors.New("no such file")
	}
	const keyVar = "APPLE_APPSTORE_API_PRIVATE_KEY"
	tests := []struct {
		name    string
		key     string
		wantErr string
		secret  string // must not appear in the error
	}{
		{name: "real newlines", key: ec},
		{name: "literal backslash n", key: strings.ReplaceAll(strings.TrimSpace(ec), "\n", `\n`)},
		{name: "double quoted", key: `"` + strings.TrimSpace(ec) + `"`},
		{name: "single quoted trailing newline", key: "'" + strings.TrimSpace(ec) + "'\n"},
		{name: "path", key: "/keys/AuthKey.p8"},
		{name: "garbage", key: "abc", wantErr: keyVar, secret: "abc"},
		{name: "rsa", key: rsa, wantErr: "not a PKCS#8 EC private key", secret: "MII"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			env := envOf(map[string]string{
				"APPLE_APPSTORE_ISSUER_ID": "issuer", "APPLE_APPSTORE_API_KEY_ID": "KEY1", keyVar: tt.key,
			})
			creds, err := config.LoadAppleCredentials(env, readFile)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Fatalf("err = %v, want %q", err, tt.wantErr)
				}
				if tt.secret != "" && strings.Contains(err.Error(), tt.secret) {
					t.Fatalf("error leaks value: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatal(err)
			}
			if creds.IssuerID != "issuer" || creds.KeyID != "KEY1" || !strings.Contains(string(creds.PrivateKeyPEM), "-----BEGIN PRIVATE KEY-----\n") {
				t.Fatalf("creds = %+v", creds)
			}
		})
	}

	t.Run("all missing", func(t *testing.T) {
		_, err := config.LoadAppleCredentials(envOf(nil), readFile)
		if err == nil {
			t.Fatal("want error")
		}
		for _, n := range []string{"APPLE_APPSTORE_ISSUER_ID", "APPLE_APPSTORE_API_KEY_ID", keyVar} {
			if !strings.Contains(err.Error(), n) {
				t.Errorf("error lacks %s: %v", n, err)
			}
		}
	})
}

func TestParseAppleKey(t *testing.T) {
	key, err := config.ParseAppleKey([]byte(readTestdata(t, "test_ec_p256.p8")))
	if err != nil || key.Curve != elliptic.P256() {
		t.Fatalf("key=%v err=%v", key, err)
	}
	for _, bad := range []string{"", "abc", readTestdata(t, "test_rsa.p8")} {
		if _, err := config.ParseAppleKey([]byte(bad)); err == nil || !strings.Contains(err.Error(), "not a PKCS#8 EC private key") {
			t.Errorf("err = %v", err)
		}
	}
}

func TestLoadGoogleServiceAccountShapes(t *testing.T) {
	const content = `{"type":"service_account","client_email":"a@b.iam.gserviceaccount.com","private_key":"k"}`
	readFile := func(p string) ([]byte, error) {
		if p == "/sa.json" {
			return []byte(content), nil
		}
		return nil, errors.New("no such file")
	}
	const v = "GOOGLE_PLAY_SERVICE_ACCOUNT_JSON"
	tests := []struct {
		name    string
		value   string
		wantErr bool
		secret  string
	}{
		{name: "content", value: content},
		{name: "content with whitespace", value: "  \n" + content + "\n "},
		{name: "path", value: "/sa.json"},
		{name: "authorized_user", value: `{"type":"authorized_user","client_email":"x","private_key":"SECRETKEY"}`, wantErr: true, secret: "SECRETKEY"},
		{name: "not json", value: "{not json SECRETTEXT", wantErr: true, secret: "SECRETTEXT"},
		{name: "missing path", value: "/nope.json", wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := config.LoadGoogleServiceAccount(envOf(map[string]string{v: tt.value}), readFile)
			if tt.wantErr {
				if err == nil || !strings.Contains(err.Error(), v) {
					t.Fatalf("err = %v", err)
				}
				if tt.secret != "" && strings.Contains(err.Error(), tt.secret) {
					t.Fatalf("error leaks value: %v", err)
				}
				return
			}
			if err != nil || string(got) != content {
				t.Fatalf("got %q err %v", got, err)
			}
		})
	}
	t.Run("unset", func(t *testing.T) {
		_, err := config.LoadGoogleServiceAccount(envOf(nil), readFile)
		if err == nil || !strings.Contains(err.Error(), v) {
			t.Fatalf("err = %v", err)
		}
	})
}
