package fakeroute

import (
	"bytes"
	"io/fs"
	"net/http"
)

// Fixtures reads the files of a fake's embedded testdata directory.
type Fixtures struct {
	FS   fs.FS  // holds testdata/
	Name string // the fake's package name, for panic messages
}

// baseURLPlaceholder in a fixture is replaced with the fake's URL, so links
// such as links.next point back at the fake.
const baseURLPlaceholder = "{{baseURL}}"

// Read returns testdata/<name>. It panics if the file does not exist.
func (f Fixtures) Read(name string) []byte {
	b, err := fs.ReadFile(f.FS, "testdata/"+name)
	if err != nil {
		panic(f.Name + ": " + err.Error())
	}
	return b
}

// JSON answers with status and the fixture testdata/<fixture>, with
// "{{baseURL}}" replaced by the fake's URL. It panics if the fixture does
// not exist.
func (f Fixtures) JSON(status int, fixture string) HandlerFunc {
	body := f.Read(fixture)
	return func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(status)
		_, _ = w.Write(bytes.ReplaceAll(body, []byte(baseURLPlaceholder), []byte("http://"+r.Host)))
	}
}
