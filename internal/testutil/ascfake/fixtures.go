package ascfake

import (
	"bytes"
	"embed"
	"net/http"

	"github.com/nikeee/store-cli/internal/config"
)

//go:embed testdata
var testdata embed.FS

// Test credentials. KeyPEM is a P-256 PKCS#8 key made for tests only.
const (
	IssuerID = "57246542-96fe-1a63-e053-0824d011072a"
	KeyID    = "TESTKEY123"
)

// KeyPEM is the private key of Credentials.
var KeyPEM = mustRead("testdata/test_ec_p256.p8")

// Credentials returns App Store Connect credentials with the test key.
func Credentials() config.AppleCredentials {
	return config.AppleCredentials{IssuerID: IssuerID, KeyID: KeyID, PrivateKeyPEM: KeyPEM}
}

// baseURLPlaceholder in a fixture is replaced with the fake's URL, so
// links.next points back at the fake.
const baseURLPlaceholder = "{{baseURL}}"

// JSON answers with status and the fixture testdata/<fixture>. It panics if
// the fixture does not exist.
func JSON(status int, fixture string) HandlerFunc {
	body := mustRead("testdata/" + fixture)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(bytes.ReplaceAll(body, []byte(baseURLPlaceholder), []byte("http://"+r.Host)))
	}
}

func mustRead(name string) []byte {
	b, err := testdata.ReadFile(name)
	if err != nil {
		panic("ascfake: " + err.Error())
	}
	return b
}
