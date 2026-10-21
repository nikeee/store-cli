package playfake

import (
	"embed"
	"net/http"
)

//go:embed testdata
var testdata embed.FS

// ClientEmail is the service account of ServiceAccountJSON.
const ClientEmail = "store-cli-test@store-cli-test.iam.gserviceaccount.com"

// ServiceAccountJSON is a service account key whose RSA PKCS#8 private key
// was made for tests only.
var ServiceAccountJSON = mustRead("testdata/service_account.json")

// JSON answers with status and the fixture testdata/<fixture>. It panics if
// the fixture does not exist.
func JSON(status int, fixture string) HandlerFunc {
	body := mustRead("testdata/" + fixture)
	return func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}
}

func mustRead(name string) []byte {
	b, err := testdata.ReadFile(name)
	if err != nil {
		panic("playfake: " + err.Error())
	}
	return b
}
