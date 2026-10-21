// Package media is the single description of Apple screenshot/preview
// devices and Google Play image types, plus offline image validation,
// checksum lists and directory scans. It does not talk to any API.
package media

import (
	"fmt"
	"strings"

	"github.com/nikeee/store-cli/internal/clierr"
)

// Device is one App Store screenshot/preview device class.
type Device struct {
	Alias       string   // CLI alias, e.g. "iphone-6.9"
	DisplayType string   // ScreenshotDisplayType, e.g. "APP_IPHONE_67"
	PreviewType string   // display type without "APP_"; empty if previews are not supported
	Platform    string   // CLI platform: ios, macos, tvos, visionos
	Sizes       [][2]int // accepted pixel sizes, portrait width x height; landscape is accepted too
}

// Devices is ordered newest first. When a size fits several devices the
// first row wins.
//
// Sizes come from Apple's "Screenshot specifications"
// (https://developer.apple.com/help/app-store-connect/reference/screenshot-specifications),
// retrieved 2026-10-21. Apple groups sizes by display; each row maps that
// display onto the App Store Connect display type.
var Devices = []Device{
	// Source: "Dynamic Island (large display)": 1260x2736, 1290x2796, 1320x2868.
	{"iphone-6.9", "APP_IPHONE_67", "IPHONE_67", "ios", [][2]int{{1260, 2736}, {1290, 2796}, {1320, 2868}}},
	// Source: "Face ID (large display)": 1284x2778, 1242x2688.
	{"iphone-6.5", "APP_IPHONE_65", "IPHONE_65", "ios", [][2]int{{1284, 2778}, {1242, 2688}}},
	// Source: "Dynamic Island (medium display)": 1179x2556, 1206x2622.
	{"iphone-6.1", "APP_IPHONE_61", "IPHONE_61", "ios", [][2]int{{1179, 2556}, {1206, 2622}}},
	// Source: "Face ID (medium display)": 1170x2532, 1125x2436, 1080x2340.
	{"iphone-5.8", "APP_IPHONE_58", "IPHONE_58", "ios", [][2]int{{1170, 2532}, {1125, 2436}, {1080, 2340}}},
	// Source: "Home Button (large display)": 1242x2208.
	{"iphone-5.5", "APP_IPHONE_55", "IPHONE_55", "ios", [][2]int{{1242, 2208}}},
	// Source: "Home Button (medium display)": 750x1334.
	{"iphone-4.7", "APP_IPHONE_47", "IPHONE_47", "ios", [][2]int{{750, 1334}}},
	// Source: "Home Button (4\" display)": 640x1096 (no status bar), 640x1136.
	{"iphone-4.0", "APP_IPHONE_40", "IPHONE_40", "ios", [][2]int{{640, 1096}, {640, 1136}}},
	// Source: "Home Button (3.5\" display)": 640x920 (no status bar), 640x960.
	{"iphone-3.5", "APP_IPHONE_35", "IPHONE_35", "ios", [][2]int{{640, 920}, {640, 960}}},
	// Source: iPad "13\"": 2064x2752, 2048x2732.
	{"ipad-13", "APP_IPAD_PRO_3GEN_129", "IPAD_PRO_3GEN_129", "ios", [][2]int{{2064, 2752}, {2048, 2732}}},
	// Source: iPad "12.9\"": 2048x2732 (same as ipad-13, so ipad-13 wins; use --device ipad-12.9).
	{"ipad-12.9", "APP_IPAD_PRO_129", "IPAD_PRO_129", "ios", [][2]int{{2048, 2732}}},
	// Source: iPad "11\"": 1488x2266, 1668x2420, 1668x2388, 1640x2360.
	{"ipad-11", "APP_IPAD_PRO_3GEN_11", "IPAD_PRO_3GEN_11", "ios", [][2]int{{1488, 2266}, {1668, 2420}, {1668, 2388}, {1640, 2360}}},
	// Source: iPad "10.5\"": 1668x2224.
	{"ipad-10.5", "APP_IPAD_105", "IPAD_105", "ios", [][2]int{{1668, 2224}}},
	// Source: iPad "9.7\"": 1536x2008 (no status bar), 1536x2048, 768x1004, 768x1024.
	{"ipad-9.7", "APP_IPAD_97", "IPAD_97", "ios", [][2]int{{1536, 2008}, {1536, 2048}, {768, 1004}, {768, 1024}}},
	// Source: "Mac": 1280x800, 1440x900, 2560x1600, 2880x1800 (16:10, listed landscape only).
	{"mac", "APP_DESKTOP", "DESKTOP", "macos", [][2]int{{800, 1280}, {900, 1440}, {1600, 2560}, {1800, 2880}}},
	// Source: "Apple TV": 1920x1080, 3840x2160.
	{"apple-tv", "APP_APPLE_TV", "APPLE_TV", "tvos", [][2]int{{1080, 1920}, {2160, 3840}}},
	// Source: "Apple Vision Pro": 3840x2160.
	{"vision-pro", "APP_APPLE_VISION_PRO", "APPLE_VISION_PRO", "visionos", [][2]int{{2160, 3840}}},
	// Source: Apple Watch "Ultra 4, Ultra 3": 422x514; "Ultra 2, Ultra": 410x502.
	{"watch-ultra", "APP_WATCH_ULTRA", "", "ios", [][2]int{{422, 514}, {410, 502}}},
	// Source: Apple Watch "Series 12, 11, 10": 416x496.
	{"watch-10", "APP_WATCH_SERIES_10", "", "ios", [][2]int{{416, 496}}},
	// Source: Apple Watch "Series 9, 8, 7": 396x484.
	{"watch-7", "APP_WATCH_SERIES_7", "", "ios", [][2]int{{396, 484}}},
	// Source: Apple Watch "Series 6, 5, 4, SE 3, SE 2, SE": 368x448.
	{"watch-4", "APP_WATCH_SERIES_4", "", "ios", [][2]int{{368, 448}}},
	// Source: Apple Watch "Series 3, 2, 1": 312x390.
	{"watch-3", "APP_WATCH_SERIES_3", "", "ios", [][2]int{{312, 390}}},
}

// DeviceByAlias returns the device with the given alias.
func DeviceByAlias(alias string) (Device, bool) {
	for _, d := range Devices {
		if d.Alias == alias {
			return d, true
		}
	}
	return Device{}, false
}

// Accepts reports whether (w,h) or (h,w) is one of the device's sizes.
func (d Device) Accepts(w, h int) bool {
	for _, s := range d.Sizes {
		if (s[0] == w && s[1] == h) || (s[0] == h && s[1] == w) {
			return true
		}
	}
	return false
}

// DetectDevice returns the first device of platform that accepts the size.
func DetectDevice(platform string, width, height int) (Device, error) {
	var sizes []string
	for _, d := range Devices {
		if d.Platform != platform {
			continue
		}
		if d.Accepts(width, height) {
			return d, nil
		}
		for _, s := range d.Sizes {
			sizes = append(sizes, fmt.Sprintf("%dx%d (%s)", s[0], s[1], d.Alias))
		}
	}
	return Device{}, clierr.New("no %s device accepts %dx%d; accepted sizes (portrait, landscape also accepted): %s",
		platform, width, height, strings.Join(sizes, ", "))
}
