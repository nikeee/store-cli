package media

import (
	"fmt"

	"github.com/nikeee/store-cli/internal/clierr"
)

func checkFormat(path string, info ImageInfo, pngOnly bool) error {
	if info.Format != "png" && info.Format != "jpeg" {
		return clierr.New("%s: unsupported format %q, use PNG or JPEG", path, info.Format)
	}
	if pngOnly && info.Format != "png" {
		return clierr.New("%s: must be a PNG", path)
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
		return clierr.New("%s: has an alpha channel, App Store Connect rejects it", path)
	}
	if !d.Accepts(info.Width, info.Height) {
		return clierr.New("%s: %dx%d is not a %s size (accepted: %s)", path, info.Width, info.Height, d.Alias, sizeList(d))
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

// ValidateGoogleImage checks format, alpha channel, dimensions and aspect ratio.
func ValidateGoogleImage(path string, t ImageType) error {
	info, err := Inspect(path)
	if err != nil {
		return err
	}
	if err := checkFormat(path, info, t.AllowAlpha); err != nil {
		return err
	}
	if info.HasAlpha && !t.AllowAlpha {
		return clierr.New("%s: has an alpha channel, Google Play rejects it for %s", path, t.Alias)
	}
	w, h := info.Width, info.Height
	if t.Width != 0 && (w != t.Width || h != t.Height) {
		return clierr.New("%s: %dx%d, %s must be exactly %dx%d", path, w, h, t.Alias, t.Width, t.Height)
	}
	short, long := w, h
	if short > long {
		short, long = long, short
	}
	if t.MinSide != 0 && short < t.MinSide {
		return clierr.New("%s: %dx%d, each side of %s must be at least %d px", path, w, h, t.Alias, t.MinSide)
	}
	if t.MaxSide != 0 && long > t.MaxSide {
		return clierr.New("%s: %dx%d, each side of %s must be at most %d px", path, w, h, t.Alias, t.MaxSide)
	}
	if t.MaxAspect != 0 && float64(long) > t.MaxAspect*float64(short) {
		if t.MaxAspect == 1 {
			return clierr.New("%s: %dx%d, %s must be square (1:1)", path, w, h, t.Alias)
		}
		return clierr.New("%s: %dx%d, the long side of %s may be at most %gx the short side", path, w, h, t.Alias, t.MaxAspect)
	}
	return nil
}
