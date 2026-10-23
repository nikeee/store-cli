package media

import (
	"bufio"
	"encoding/binary"
	"errors"
	"image"
	_ "image/jpeg" // register decoders for image.DecodeConfig
	_ "image/png"
	"io"
	"os"

	"github.com/nikeee/store-cli/internal/clierr"
)

// ImageInfo is what the local checks need to know about an image file.
type ImageInfo struct {
	Width, Height int
	HasAlpha      bool
	Format        string // "png" or "jpeg"
}

// Inspect reads only the header (for PNGs the chunks up to the image data).
func Inspect(path string) (ImageInfo, error) {
	f, err := os.Open(path)
	if err != nil {
		return ImageInfo{}, clierr.New("%v", err)
	}
	defer func() { _ = f.Close() }()
	cfg, format, err := image.DecodeConfig(f)
	if err != nil {
		return ImageInfo{}, clierr.Filef(path, "not a PNG or JPEG image: %v", err)
	}
	info := ImageInfo{Width: cfg.Width, Height: cfg.Height, Format: format}
	if format != "png" {
		return info, nil
	}
	if _, err := f.Seek(0, io.SeekStart); err != nil {
		return ImageInfo{}, clierr.New("%v", err)
	}
	if info.HasAlpha, err = pngHasAlpha(bufio.NewReader(f)); err != nil {
		return ImageInfo{}, clierr.Filef(path, "%v", err)
	}
	return info, nil
}

// pngHasAlpha reports whether the PNG in r has an alpha channel (color type
// 4 or 6) or transparency (a tRNS chunk). The color model of
// image.DecodeConfig cannot tell: it is RGBA for opaque truecolor as well.
func pngHasAlpha(r io.Reader) (bool, error) {
	if _, err := io.CopyN(io.Discard, r, 8); err != nil { // signature
		return false, err
	}
	var head [8]byte
	for {
		if _, err := io.ReadFull(r, head[:]); err != nil {
			return false, err
		}
		n := int64(binary.BigEndian.Uint32(head[:4]))
		switch string(head[4:]) {
		case "IDAT", "IEND":
			return false, nil
		case "tRNS":
			return true, nil
		case "IHDR":
			var ihdr [13]byte
			if n != int64(len(ihdr)) {
				return false, errors.New("invalid IHDR chunk")
			}
			if _, err := io.ReadFull(r, ihdr[:]); err != nil {
				return false, err
			}
			if colorType := ihdr[9]; colorType == 4 || colorType == 6 {
				return true, nil
			}
			n = 0
		}
		if _, err := io.CopyN(io.Discard, r, n+4); err != nil { // data and CRC
			return false, err
		}
	}
}
