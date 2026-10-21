// Package golden compares test output with files in testdata/.
package golden

import (
	"flag"
	"os"
	"path/filepath"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files")

// Assert compares got with testdata/<name>.golden; with -update it rewrites the file.
func Assert(t *testing.T, name string, got []byte) {
	t.Helper()
	path := filepath.Join("testdata", name+".golden")
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, got, 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read golden file (run with -update to create it): %v", err)
	}
	if string(want) != string(got) {
		t.Errorf("%s differs from golden file %s\n--- got ---\n%s\n--- want ---\n%s", name, path, got, want)
	}
}
