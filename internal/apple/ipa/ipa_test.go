package ipa

import (
	"archive/zip"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"howett.net/plist"
)

type entry struct {
	name string
	info map[string]any
}

func writeIPA(t *testing.T, format int, entries ...entry) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "app.ipa")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	for _, e := range entries {
		data, err := plist.Marshal(e.info, format)
		if err != nil {
			t.Fatal(err)
		}
		w, err := zw.Create(e.name)
		if err != nil {
			t.Fatal(err)
		}
		if _, err := w.Write(data); err != nil {
			t.Fatal(err)
		}
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	if err := f.Close(); err != nil {
		t.Fatal(err)
	}
	return path
}

func fullInfo() map[string]any {
	return map[string]any{
		"CFBundleIdentifier":         "com.example.app",
		"CFBundleShortVersionString": "1.2.3",
		"CFBundleVersion":            "45",
		"MinimumOSVersion":           "16.0",
		"DTPlatformName":             "iphoneos",
	}
}

func want() Info {
	return Info{BundleID: "com.example.app", Version: "1.2.3", Build: "45", MinOS: "16.0", Platform: "IOS"}
}

func TestReadBinaryPlist(t *testing.T) {
	path := writeIPA(t, plist.BinaryFormat, entry{"Payload/A.app/Info.plist", fullInfo()})
	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want() {
		t.Errorf("got %+v, want %+v", got, want())
	}
}

func TestReadXMLPlist(t *testing.T) {
	path := writeIPA(t, plist.XMLFormat, entry{"Payload/A.app/Info.plist", fullInfo()})
	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if got != want() {
		t.Errorf("got %+v, want %+v", got, want())
	}
}

func TestPlatformMapping(t *testing.T) {
	tests := map[string]string{
		"iphoneos": "IOS", "appletvos": "TV_OS", "xros": "VISION_OS", "": "IOS",
	}
	for in, out := range tests {
		info := fullInfo()
		if in == "" {
			delete(info, "DTPlatformName")
		} else {
			info["DTPlatformName"] = in
		}
		got, err := Read(writeIPA(t, plist.BinaryFormat, entry{"Payload/A.app/Info.plist", info}))
		if err != nil {
			t.Fatal(err)
		}
		if got.Platform != out {
			t.Errorf("%q: got %q, want %q", in, got.Platform, out)
		}
	}
}

// A simulator or other unknown platform is no App Store build.
func TestUnknownPlatformIsAnError(t *testing.T) {
	info := fullInfo()
	info["DTPlatformName"] = "iphonesimulator"
	_, err := Read(writeIPA(t, plist.BinaryFormat, entry{"Payload/A.app/Info.plist", info}))
	if err == nil || !strings.Contains(err.Error(), `DTPlatformName "iphonesimulator"`) {
		t.Fatalf("err = %v", err)
	}
}

func TestIgnoresNestedPlists(t *testing.T) {
	other := fullInfo()
	other["CFBundleIdentifier"] = "com.example.app.widget"
	path := writeIPA(t, plist.BinaryFormat,
		entry{"Payload/A.app/PlugIns/W.appex/Info.plist", other},
		entry{"Payload/A.app/Info.plist", fullInfo()},
		entry{"Payload/A.app/Frameworks/F.framework/Info.plist", other},
	)
	got, err := Read(path)
	if err != nil {
		t.Fatal(err)
	}
	if got.BundleID != "com.example.app" {
		t.Errorf("got %q", got.BundleID)
	}
}

func TestMissingBuildNumber(t *testing.T) {
	info := fullInfo()
	delete(info, "CFBundleVersion")
	path := writeIPA(t, plist.BinaryFormat, entry{"Payload/A.app/Info.plist", info})
	_, err := Read(path)
	if err == nil || !strings.Contains(err.Error(), "CFBundleVersion") || !strings.Contains(err.Error(), path) {
		t.Fatalf("got %v", err)
	}
}

func TestNoOrSeveralAppPlists(t *testing.T) {
	none := writeIPA(t, plist.BinaryFormat, entry{"Payload/A.app/PlugIns/W.appex/Info.plist", fullInfo()})
	if _, err := Read(none); err == nil {
		t.Error("expected error for no app plist")
	}
	two := writeIPA(t, plist.BinaryFormat,
		entry{"Payload/A.app/Info.plist", fullInfo()},
		entry{"Payload/B.app/Info.plist", fullInfo()},
	)
	if _, err := Read(two); err == nil {
		t.Error("expected error for several app plists")
	}
}

func TestNotAZip(t *testing.T) {
	path := filepath.Join(t.TempDir(), "x.ipa")
	if err := os.WriteFile(path, []byte("hello"), 0o600); err != nil {
		t.Fatal(err)
	}
	_, err := Read(path)
	if err == nil || err.Error() != path+" is not an IPA (not a zip archive)" {
		t.Fatalf("got %v", err)
	}
}

func TestMissingFileNamesThePathOnce(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "app.ipa")
	_, err := Read(missing)
	if err == nil || strings.Count(err.Error(), missing) != 1 {
		t.Errorf("err = %v", err)
	}
}

func TestUndecodablePlist(t *testing.T) {
	path := filepath.Join(t.TempDir(), "app.ipa")
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	zw := zip.NewWriter(f)
	w, err := zw.Create("Payload/App.app/Info.plist")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := w.Write([]byte("bplist00 garbage")); err != nil {
		t.Fatal(err)
	}
	if err := zw.Close(); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	if _, err := Read(path); err == nil || !strings.Contains(err.Error(), "decode Payload/App.app/Info.plist") {
		t.Errorf("err = %v", err)
	}
}
