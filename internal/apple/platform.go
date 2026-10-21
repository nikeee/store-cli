package apple

import "github.com/nikeee/store-cli/internal/apple/asc"

// platforms maps the --platform values to App Store Connect platforms.
var platforms = map[string]asc.Platform{
	"ios":      asc.PlatformIos,
	"macos":    asc.PlatformMacOs,
	"tvos":     asc.PlatformTvOs,
	"visionos": asc.PlatformVisionOs,
}

// APIPlatform returns the App Store Connect platform for a --platform value.
func APIPlatform(flag string) (asc.Platform, bool) {
	p, ok := platforms[flag]
	return p, ok
}

// flagPlatform returns the --platform value for an App Store Connect
// platform, so results can be fed back into --platform. Unknown platforms
// keep the API value.
func flagPlatform(p asc.Platform) string {
	for flag, api := range platforms {
		if api == p {
			return flag
		}
	}
	return string(p)
}
