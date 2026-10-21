package ascfake

import (
	"embed"

	"github.com/nikeee/store-cli/internal/config"
	"github.com/nikeee/store-cli/internal/testutil/fakeroute"
)

//go:embed testdata
var testdata embed.FS

var fixtures = fakeroute.Fixtures{FS: testdata, Name: "ascfake"}

// Test credentials. KeyPEM is a P-256 PKCS#8 key made for tests only.
const (
	IssuerID = "57246542-96fe-1a63-e053-0824d011072a"
	KeyID    = "TESTKEY123"
)

// KeyPEM is the private key of Credentials.
var KeyPEM = fixtures.Read("test_ec_p256.p8")

// Credentials returns App Store Connect credentials with the test key.
func Credentials() config.AppleCredentials {
	return config.AppleCredentials{IssuerID: IssuerID, KeyID: KeyID, PrivateKeyPEM: KeyPEM}
}

// JSON answers with status and the fixture testdata/<fixture>. "{{baseURL}}"
// in the fixture is replaced with the fake's URL, so links.next points back
// at the fake. It panics if the fixture does not exist.
func JSON(status int, fixture string) HandlerFunc {
	return fixtures.JSON(status, fixture)
}
