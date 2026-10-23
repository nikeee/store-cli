// Package filetree writes directory trees for tests.
package filetree

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/testutil/pngfake"
)

// Write writes files (slash path relative to root -> content) below root. A
// path ending in .png gets an opaque PNG described by its content, "WxH" or
// "WxH/shade"; every other file gets the content as is. It returns the
// SHA-256 (hex) of every PNG by its path.
func Write(t testing.TB, root string, files map[string]string) map[string]string {
	t.Helper()
	sums := map[string]string{}
	for p, c := range files {
		full := filepath.Join(root, filepath.FromSlash(p))
		if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(p, ".png") {
			if err := os.WriteFile(full, []byte(c), 0o600); err != nil {
				t.Fatal(err)
			}
			continue
		}
		var w, h int
		shade := uint8(1)
		if _, err := fmt.Sscanf(c, "%dx%d/%d", &w, &h, &shade); err != nil {
			if _, err := fmt.Sscanf(c, "%dx%d", &w, &h); err != nil {
				t.Fatalf("%s: %q is no PNG size", p, c)
			}
		}
		pngfake.Write(t, full, w, h, shade)
		b, err := os.ReadFile(full)
		if err != nil {
			t.Fatal(err)
		}
		sum := sha256.Sum256(b)
		sums[p] = hex.EncodeToString(sum[:])
	}
	return sums
}
