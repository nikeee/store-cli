// Package pngfake writes small PNG files for tests.
package pngfake

import (
	"image"
	"image/color"
	"image/png"
	"os"
	"testing"
)

// Write writes an opaque grayscale PNG of w x h pixels to path. Different
// shades give different file contents (and so different checksums).
func Write(t testing.TB, path string, w, h int, shade uint8) {
	t.Helper()
	img := image.NewGray(image.Rect(0, 0, w, h))
	for i := range img.Pix {
		img.Pix[i] = shade
	}
	img.SetGray(0, 0, color.Gray{Y: shade})
	encode(t, path, img)
}

// WriteAlpha writes a PNG with an alpha channel.
func WriteAlpha(t testing.TB, path string, w, h int) {
	t.Helper()
	encode(t, path, image.NewNRGBA(image.Rect(0, 0, w, h)))
}

func encode(t testing.TB, path string, img image.Image) {
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
