package playfake

import (
	"embed"

	"github.com/nikeee/store-cli/internal/testutil/fakeroute"
)

//go:embed testdata
var testdata embed.FS

var fixtures = fakeroute.Fixtures{FS: testdata, Name: "playfake"}

// ClientEmail is the service account of ServiceAccountJSON.
const ClientEmail = "store-cli-test@store-cli-test.iam.gserviceaccount.com"

// ServiceAccountJSON is a service account key whose RSA PKCS#8 private key
// was made for tests only.
var ServiceAccountJSON = fixtures.Read("service_account.json")

// JSON answers with status and the fixture testdata/<fixture>. It panics if
// the fixture does not exist.
func JSON(status int, fixture string) HandlerFunc {
	return fixtures.JSON(status, fixture)
}
