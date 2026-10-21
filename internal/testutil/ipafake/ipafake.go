// Package ipafake writes small IPA files for tests.
package ipafake

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"howett.net/plist"
)

// Spec describes the IPA to write. Size, if positive, is the exact size of
// the file in bytes (it must not be smaller than the minimal IPA).
type Spec struct {
	BundleID string
	Version  string
	Build    string
	Platform string // DTPlatformName, default iphoneos
	Size     int
}

// Write writes an IPA with an Info.plist per spec to a temp dir and returns
// its path (named app.ipa).
func Write(t testing.TB, s Spec) string {
	t.Helper()
	if s.Platform == "" {
		s.Platform = "iphoneos"
	}
	data, err := plist.Marshal(map[string]any{
		"CFBundleIdentifier":         s.BundleID,
		"CFBundleShortVersionString": s.Version,
		"CFBundleVersion":            s.Build,
		"MinimumOSVersion":           "16.0",
		"DTPlatformName":             s.Platform,
	}, plist.XMLFormat)
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(t.TempDir(), "app.ipa")
	write(t, path, data, "")
	if s.Size > 0 {
		fi, err := os.Stat(path)
		if err != nil {
			t.Fatal(err)
		}
		pad := s.Size - int(fi.Size())
		if pad < 0 {
			t.Fatalf("ipafake: minimal IPA is %d bytes, Size is %d", fi.Size(), s.Size)
		}
		// The archive comment adds exactly its length to the file.
		write(t, path, data, strings.Repeat("x", pad))
	}
	return path
}

func write(t testing.TB, path string, plistData []byte, comment string) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	if err := zw.SetComment(comment); err != nil {
		t.Fatal(err)
	}
	w, err := zw.Create("Payload/App.app/Info.plist")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write(plistData); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
}
