package media

import (
	"image"
	"image/color"
	"image/jpeg"
	"image/png"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/apple/asc"
)

func writePNG(t *testing.T, path string, img image.Image) {
	t.Helper()
	f, err := os.Create(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = f.Close() }()
	if err := png.Encode(f, img); err != nil {
		t.Fatal(err)
	}
}

func writeSized(t *testing.T, path string, w, h int) {
	t.Helper()
	writePNG(t, path, image.NewRGBA64(image.Rect(0, 0, w, h)))
}

func writeOpaque(t *testing.T, path string, w, h int) {
	t.Helper()
	writePNG(t, path, image.NewGray(image.Rect(0, 0, w, h)))
}

func TestDetectDevice(t *testing.T) {
	tests := []struct {
		platform string
		w, h     int
		want     string
		wantErr  bool
	}{
		{"ios", 1284, 2778, "iphone-6.5", false},
		{"ios", 2778, 1284, "iphone-6.5", false},
		{"tvos", 3840, 2160, "apple-tv", false},
		{"visionos", 3840, 2160, "vision-pro", false},
		{"ios", 1000, 1000, "", true},
	}
	for _, tt := range tests {
		d, err := DetectDevice(tt.platform, tt.w, tt.h)
		if tt.wantErr {
			if err == nil || !strings.Contains(err.Error(), "1284x2778") {
				t.Errorf("%s %dx%d: want error listing sizes, got %v", tt.platform, tt.w, tt.h, err)
			}
			continue
		}
		if err != nil || d.Alias != tt.want {
			t.Errorf("%s %dx%d: got %q, %v; want %q", tt.platform, tt.w, tt.h, d.Alias, err, tt.want)
		}
	}
}

func TestDevicesMatchASCEnums(t *testing.T) {
	display := map[asc.ScreenshotDisplayType]bool{}
	for _, v := range []asc.ScreenshotDisplayType{
		asc.ScreenshotDisplayTypeAppIphone67, asc.ScreenshotDisplayTypeAppIphone61, asc.ScreenshotDisplayTypeAppIphone65,
		asc.ScreenshotDisplayTypeAppIphone58, asc.ScreenshotDisplayTypeAppIphone55, asc.ScreenshotDisplayTypeAppIphone47,
		asc.ScreenshotDisplayTypeAppIphone40, asc.ScreenshotDisplayTypeAppIphone35, asc.ScreenshotDisplayTypeAppIpadPro3gen129,
		asc.ScreenshotDisplayTypeAppIpadPro3gen11, asc.ScreenshotDisplayTypeAppIpadPro129, asc.ScreenshotDisplayTypeAppIpad105,
		asc.ScreenshotDisplayTypeAppIpad97, asc.ScreenshotDisplayTypeAppDesktop, asc.ScreenshotDisplayTypeAppWatchUltra,
		asc.ScreenshotDisplayTypeAppWatchSeries10, asc.ScreenshotDisplayTypeAppWatchSeries7, asc.ScreenshotDisplayTypeAppWatchSeries4,
		asc.ScreenshotDisplayTypeAppWatchSeries3, asc.ScreenshotDisplayTypeAppAppleTv, asc.ScreenshotDisplayTypeAppAppleVisionPro,
	} {
		display[v] = true
	}
	preview := map[asc.PreviewType]bool{}
	for _, v := range []asc.PreviewType{
		asc.PreviewTypeIphone67, asc.PreviewTypeIphone61, asc.PreviewTypeIphone65, asc.PreviewTypeIphone58,
		asc.PreviewTypeIphone55, asc.PreviewTypeIphone47, asc.PreviewTypeIphone40, asc.PreviewTypeIphone35,
		asc.PreviewTypeIpadPro3gen129, asc.PreviewTypeIpadPro3gen11, asc.PreviewTypeIpadPro129, asc.PreviewTypeIpad105,
		asc.PreviewTypeIpad97, asc.PreviewTypeDesktop, asc.PreviewTypeAppleTv, asc.PreviewTypeAppleVisionPro,
	} {
		preview[v] = true
	}
	for _, d := range Devices {
		if !display[asc.ScreenshotDisplayType(d.DisplayType)] {
			t.Errorf("%s: display type %q not in asc.ScreenshotDisplayType", d.Alias, d.DisplayType)
		}
		if d.PreviewType != "" && !preview[asc.PreviewType(d.PreviewType)] {
			t.Errorf("%s: preview type %q not in asc.PreviewType", d.Alias, d.PreviewType)
		}
		if len(d.Sizes) == 0 {
			t.Errorf("%s: no sizes", d.Alias)
		}
	}
}

func TestInspectAlpha(t *testing.T) {
	dir := t.TempDir()
	r := image.Rect(0, 0, 4, 3)
	pal := image.NewPaletted(r, color.Palette{color.RGBA{A: 0}, color.RGBA{R: 255, A: 255}})
	opaquePal := image.NewPaletted(r, color.Palette{color.RGBA{A: 255}, color.RGBA{R: 255, A: 255}})
	tests := []struct {
		name string
		img  image.Image
		want bool
	}{
		{"nrgba", image.NewNRGBA(r), true},
		{"rgba", image.NewRGBA(r), true},
		{"gray", image.NewGray(r), false},
		{"pal-transparent", pal, true},
		{"pal-opaque", opaquePal, false},
	}
	for _, tt := range tests {
		p := filepath.Join(dir, tt.name+".png")
		writePNG(t, p, tt.img)
		info, err := Inspect(p)
		if err != nil {
			t.Fatal(err)
		}
		if info.HasAlpha != tt.want || info.Width != 4 || info.Height != 3 || info.Format != "png" {
			t.Errorf("%s: got %+v, want alpha=%v", tt.name, info, tt.want)
		}
	}
	p := filepath.Join(dir, "a.jpg")
	f, _ := os.Create(p)
	if err := jpeg.Encode(f, image.NewRGBA(r), nil); err != nil {
		t.Fatal(err)
	}
	_ = f.Close()
	info, err := Inspect(p)
	if err != nil || info.HasAlpha || info.Format != "jpeg" {
		t.Errorf("jpeg: %+v, %v", info, err)
	}
}

func TestValidateAppleScreenshot(t *testing.T) {
	dir := t.TempDir()
	d, _ := DeviceByAlias("iphone-6.5")
	ok := filepath.Join(dir, "ok.png")
	writeOpaque(t, ok, 1284, 2778)
	if err := ValidateAppleScreenshot(ok, d); err != nil {
		t.Error(err)
	}
	alpha := filepath.Join(dir, "alpha.png")
	writeSized(t, alpha, 1284, 2778)
	err := ValidateAppleScreenshot(alpha, d)
	if err == nil || err.Error() != alpha+": has an alpha channel, App Store Connect rejects it" {
		t.Errorf("alpha: %v", err)
	}
	bad := filepath.Join(dir, "bad.png")
	writeOpaque(t, bad, 100, 100)
	if err := ValidateAppleScreenshot(bad, d); err == nil {
		t.Error("want size error")
	}
}

func TestValidateGoogleImageAspect(t *testing.T) {
	dir := t.TempDir()
	tests := []struct {
		typ     string
		w, h    int
		wantErr bool
	}{
		{"phone", 1080, 2400, true},
		{"phone", 1080, 1920, false},
		{"phone", 300, 600, true},
		{"feature-graphic", 1024, 500, false},
		{"feature-graphic", 1000, 500, true},
		{"icon", 512, 512, false},
		{"wear", 384, 384, false},
		{"wear", 384, 500, true},
	}
	for _, tt := range tests {
		typ, ok := ImageTypeByAlias(tt.typ)
		if !ok {
			t.Fatalf("no type %s", tt.typ)
		}
		p := filepath.Join(dir, "x.png")
		writeOpaque(t, p, tt.w, tt.h)
		if tt.typ == "icon" {
			writeSized(t, p, tt.w, tt.h) // icon allows alpha
		}
		err := ValidateGoogleImage(p, typ)
		if (err != nil) != tt.wantErr {
			t.Errorf("%s %dx%d: err=%v, wantErr=%v", tt.typ, tt.w, tt.h, err, tt.wantErr)
		}
	}
	typ, _ := ImageTypeByAlias("phone")
	p := filepath.Join(dir, "alpha.png")
	writeSized(t, p, 1080, 1920)
	if err := ValidateGoogleImage(p, typ); err == nil {
		t.Error("phone screenshot with alpha must fail")
	}
}

func TestScanAppleScreenshots(t *testing.T) {
	root := t.TempDir()
	loc := filepath.Join(root, "en-US")
	if err := os.MkdirAll(filepath.Join(loc, "ipad-13"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeOpaque(t, filepath.Join(loc, "2.png"), 1284, 2778)
	writeOpaque(t, filepath.Join(loc, "1.png"), 1284, 2778)
	writeOpaque(t, filepath.Join(loc, ".hidden.png"), 1284, 2778)
	writeOpaque(t, filepath.Join(loc, "3.JPG"), 1284, 2778)
	if err := os.WriteFile(filepath.Join(loc, "notes.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	writeOpaque(t, filepath.Join(loc, "ipad-13", "b.png"), 1, 1)
	writeOpaque(t, filepath.Join(loc, "ipad-13", "a.png"), 1, 1)
	got, err := ScanAppleScreenshots(root, "ios")
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string][]string{"en-US": {
		"iphone-6.5": {filepath.Join(loc, "1.png"), filepath.Join(loc, "2.png"), filepath.Join(loc, "3.JPG")},
		"ipad-13":    {filepath.Join(loc, "ipad-13", "a.png"), filepath.Join(loc, "ipad-13", "b.png")},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
	if got, err := ScanAppleScreenshots(filepath.Join(root, "missing"), "ios"); err != nil || len(got) != 0 {
		t.Errorf("missing dir: %v, %v", got, err)
	}
}

func TestScanGoogleImages(t *testing.T) {
	root := t.TempDir()
	imgs := filepath.Join(root, "android", "en-US", "images")
	if err := os.MkdirAll(filepath.Join(imgs, "phoneScreenshots"), 0o755); err != nil {
		t.Fatal(err)
	}
	writeOpaque(t, filepath.Join(imgs, "phoneScreenshots", "b.png"), 1, 1)
	writeOpaque(t, filepath.Join(imgs, "phoneScreenshots", "a.png"), 1, 1)
	writeOpaque(t, filepath.Join(imgs, "featureGraphic.png"), 1, 1)
	got, err := ScanGoogleImages(root)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]map[string][]string{"en-US": {
		"phone":           {filepath.Join(imgs, "phoneScreenshots", "a.png"), filepath.Join(imgs, "phoneScreenshots", "b.png")},
		"feature-graphic": {filepath.Join(imgs, "featureGraphic.png")},
	}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("got %v\nwant %v", got, want)
	}
}

func TestChecksumListsOrder(t *testing.T) {
	dir := t.TempDir()
	a, b := filepath.Join(dir, "a"), filepath.Join(dir, "b")
	if err := os.WriteFile(a, []byte("a"), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(b, []byte("b"), 0o644); err != nil {
		t.Fatal(err)
	}
	md5, err := MD5List([]string{b, a})
	if err != nil {
		t.Fatal(err)
	}
	if want := (ChecksumList{"92eb5ffee6ae2fec3ad71c777531578f", "0cc175b9c0f1b6a831c399e269772661"}); !reflect.DeepEqual(md5, want) {
		t.Errorf("md5 %v", md5)
	}
	sha, err := SHA256List([]string{a, b})
	if err != nil {
		t.Fatal(err)
	}
	want := ChecksumList{
		"ca978112ca1bbdcafac231b39a23dc4da786eff8147c4e72b9807785afee48bb",
		"3e23e8160039594a33894f6564e1b1348bbd7a0088d42c4acb73eeaed59c009d",
	}
	if !reflect.DeepEqual(sha, want) {
		t.Errorf("sha %v", sha)
	}
	if _, err := MD5List([]string{filepath.Join(dir, "nope")}); err == nil {
		t.Error("want error")
	}
}

// The *PathError of a failed open or read already names the file; the
// message must not repeat it.
func TestFileErrorsNameThePathOnce(t *testing.T) {
	dir := t.TempDir()
	missing := filepath.Join(dir, "missing.png")
	for name, c := range map[string]struct {
		path string
		run  func(string) error
	}{
		"inspect missing": {missing, func(p string) error { _, err := Inspect(p); return err }},
		"md5 missing":     {missing, func(p string) error { _, err := MD5List([]string{p}); return err }},
		"sha256 of a dir": {dir, func(p string) error { _, err := SHA256List([]string{p}); return err }},
	} {
		err := c.run(c.path)
		if err == nil || strings.Count(err.Error(), c.path) != 1 {
			t.Errorf("%s: err = %v", name, err)
		}
	}
}
