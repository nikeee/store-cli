package media

import (
	"fmt"
	"os"

	"github.com/nikeee/store-cli/internal/clierr"
)

func checkFormat(path string, info ImageInfo, pngOnly bool) error {
	if info.Format != "png" && info.Format != "jpeg" {
		return clierr.Filef(path, "unsupported format %q, use PNG or JPEG", info.Format)
	}
	if pngOnly && info.Format != "png" {
		return clierr.Filef(path, "must be a PNG")
	}
	return nil
}

// ValidateAppleScreenshot checks format, alpha channel and pixel size.
func ValidateAppleScreenshot(path string, d Device) error {
	info, err := Inspect(path)
	if err != nil {
		return err
	}
	if err := checkFormat(path, info, false); err != nil {
		return err
	}
	if info.HasAlpha {
		return clierr.Filef(path, "has an alpha channel, App Store Connect rejects it")
	}
	if !d.Accepts(info.Width, info.Height) {
		return clierr.Filef(path, "%dx%d is not a %s size (accepted: %s)", info.Width, info.Height, d.Alias, sizeList(d))
	}
	return nil
}

func sizeList(d Device) string {
	s := ""
	for i, sz := range d.Sizes {
		if i > 0 {
			s += ", "
		}
		s += fmt.Sprintf("%dx%d", sz[0], sz[1])
	}
	return s
}

// ValidateGoogleImage checks format, alpha channel, dimensions, aspect ratio
// and file size.
func ValidateGoogleImage(path string, t ImageType) error {
	info, err := Inspect(path)
	if err != nil {
		return err
	}
	if err := checkFormat(path, info, t.AllowAlpha); err != nil {
		return err
	}
	if info.HasAlpha && !t.AllowAlpha {
		return clierr.Filef(path, "has an alpha channel, Google Play rejects it for %s", t.Alias)
	}
	w, h := info.Width, info.Height
	if t.Width != 0 && (w != t.Width || h != t.Height) {
		return clierr.Filef(path, "%dx%d, %s must be exactly %dx%d", w, h, t.Alias, t.Width, t.Height)
	}
	short, long := w, h
	if short > long {
		short, long = long, short
	}
	if t.MinSide != 0 && short < t.MinSide {
		return clierr.Filef(path, "%dx%d, each side of %s must be at least %d px", w, h, t.Alias, t.MinSide)
	}
	if t.MaxSide != 0 && long > t.MaxSide {
		return clierr.Filef(path, "%dx%d, each side of %s must be at most %d px", w, h, t.Alias, t.MaxSide)
	}
	if t.MaxAspect != 0 && float64(long) > t.MaxAspect*float64(short) {
		if t.MaxAspect == 1 {
			return clierr.Filef(path, "%dx%d, %s must be square (1:1)", w, h, t.Alias)
		}
		return clierr.Filef(path, "%dx%d, the long side of %s may be at most %gx the short side", w, h, t.Alias, t.MaxAspect)
	}
	if t.MaxBytes == 0 {
		return nil
	}
	st, err := os.Stat(path)
	if err != nil {
		return clierr.New("%v", err)
	}
	if st.Size() > t.MaxBytes {
		return clierr.Filef(path, "%d KB, %s may be at most %d KB", (st.Size()+1023)/1024, t.Alias, t.MaxBytes/1024)
	}
	return nil
}
