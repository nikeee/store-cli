package media

import (
	"image"
	"image/color"
	_ "image/jpeg" // register decoders for image.DecodeConfig
	_ "image/png"
	"os"

	"github.com/nikeee/store-cli/internal/clierr"
)

// ImageInfo is what the local checks need to know about an image file.
type ImageInfo struct {
	Width, Height int
	HasAlpha      bool
	Format        string // "png" or "jpeg"
}

// Inspect reads only the header (plus the palette for paletted PNGs).
func Inspect(path string) (ImageInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return ImageInfo{}, clierr.New("open %s: %v", path, err)
	}
	defer func() { _ = f.Close() }()
	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return ImageInfo{}, clierr.New("%s: not a PNG or JPEG image: %v", path, err)
	}
	return ImageInfo{Width: cfg.Width, Height: cfg.Height, HasAlpha: hasAlpha(cfg.ColorModel), Format: format}, nil
}

func hasAlpha(m color.Model) bool {
	switch m {
	case color.NRGBAModel, color.RGBAModel, color.NRGBA64Model, color.RGBA64Model, color.AlphaModel, color.Alpha16Model:
		return true
	}
	if p, ok := m.(color.Palette); ok {
		for _, c := range p {
			if _, _, _, a := c.RGBA(); a != 0xffff {
				return true
			}
		}
	}
	return false
}
