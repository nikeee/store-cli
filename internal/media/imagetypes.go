package media

// ImageType is one Google Play listing image slot.
type ImageType struct {
	Alias    string // CLI alias, e.g. "phone"
	APIType  string // androidpublisher image type, e.g. "phoneScreenshots"
	Path     string // below <locale>/ in the metadata directory: a folder if Multiple, else a file
	Multiple bool
	Max      int
	// Width and Height are exact required pixel sizes; 0 means flexible.
	Width, Height int
	// MinSide and MaxSide limit each side when Width/Height are 0.
	MinSide, MaxSide int
	// MaxAspect limits long side / short side; 0 = no limit.
	MaxAspect float64
	// AllowAlpha is true only where Play accepts a transparent image.
	AllowAlpha bool
}

// ImageTypes lists the Play image types in the order of docs/reference/tables.md.
//
// Limits come from Play Console help "Add preview assets to showcase your app"
// (https://support.google.com/googleplay/android-developer/answer/9866151),
// retrieved 2026-10-21:
//   - screenshots: JPEG or 24-bit PNG without alpha, each side 320..3840 px,
//     long side at most 2x the short side, at most 8 per type;
//   - wear screenshots: 1:1, at least 384x384 px;
//   - feature graphic: 1024x500, JPEG or 24-bit PNG without alpha;
//   - icon: 512x512 32-bit PNG (alpha allowed);
//   - TV banner: 1280x720, JPEG or 24-bit PNG without alpha;
//   - promo graphic: not on that page; 180x120 is the legacy Play Console
//     requirement (JPEG or 24-bit PNG without alpha).
var ImageTypes = []ImageType{
	{Alias: "phone", APIType: "phoneScreenshots", Path: "images/phoneScreenshots", Multiple: true, Max: 8, MinSide: 320, MaxSide: 3840, MaxAspect: 2},
	{Alias: "tablet-7", APIType: "sevenInchScreenshots", Path: "images/sevenInchScreenshots", Multiple: true, Max: 8, MinSide: 320, MaxSide: 3840, MaxAspect: 2},
	{Alias: "tablet-10", APIType: "tenInchScreenshots", Path: "images/tenInchScreenshots", Multiple: true, Max: 8, MinSide: 320, MaxSide: 3840, MaxAspect: 2},
	{Alias: "tv", APIType: "tvScreenshots", Path: "images/tvScreenshots", Multiple: true, Max: 8, MinSide: 320, MaxSide: 3840, MaxAspect: 2},
	{Alias: "wear", APIType: "wearScreenshots", Path: "images/wearScreenshots", Multiple: true, Max: 8, MinSide: 384, MaxSide: 3840, MaxAspect: 1},
	{Alias: "feature-graphic", APIType: "featureGraphic", Path: "images/featureGraphic.png", Max: 1, Width: 1024, Height: 500},
	{Alias: "icon", APIType: "icon", Path: "images/icon.png", Max: 1, Width: 512, Height: 512, AllowAlpha: true},
	{Alias: "promo-graphic", APIType: "promoGraphic", Path: "images/promoGraphic.png", Max: 1, Width: 180, Height: 120},
	{Alias: "tv-banner", APIType: "tvBanner", Path: "images/tvBanner.png", Max: 1, Width: 1280, Height: 720},
}

// ImageTypeByAlias returns the image type with the given alias.
func ImageTypeByAlias(alias string) (ImageType, bool) {
	for _, t := range ImageTypes {
		if t.Alias == alias {
			return t, true
		}
	}
	return ImageType{}, false
}
