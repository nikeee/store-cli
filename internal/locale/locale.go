// Package locale maps App Store locales to Google Play listing languages.
package locale

// toPlay holds the App Store locales whose Play language code differs; every
// other locale is the same in both stores. Sources:
// App Store: https://developer.apple.com/documentation/appstoreconnectapi/managing-metadata-in-your-app-by-using-locale-shortcodes
// Play: https://support.google.com/googleplay/android-developer/answer/9844778 ("View list of available languages")
// Odia (or-IN) has no Play language and stays unmapped.
var toPlay = map[string]string{
	"ar-SA":   "ar",
	"cs":      "cs-CZ",
	"da":      "da-DK",
	"el":      "el-GR",
	"es-MX":   "es-419",
	"fi":      "fi-FI",
	"gu-IN":   "gu",
	"he":      "iw-IL",
	"hi":      "hi-IN",
	"hu":      "hu-HU",
	"it":      "it-IT",
	"ja":      "ja-JP",
	"ko":      "ko-KR",
	"no":      "no-NO",
	"pa-IN":   "pa",
	"pl":      "pl-PL",
	"ru":      "ru-RU",
	"sl-SI":   "sl",
	"sv":      "sv-SE",
	"tr":      "tr-TR",
	"ur-PK":   "ur",
	"zh-Hans": "zh-CN",
	"zh-Hant": "zh-TW",
}

var fromPlay = invert(toPlay)

func invert(m map[string]string) map[string]string {
	inv := make(map[string]string, len(m))
	for k, v := range m {
		inv[v] = k
	}
	return inv
}

// ToPlay returns the Play language code for an App Store locale. Play codes
// and locales that are the same in both stores pass unchanged.
func ToPlay(locale string) string {
	if p, ok := toPlay[locale]; ok {
		return p
	}
	return locale
}

// FromPlay returns the App Store locale for a Play language code, the
// inverse of ToPlay. Codes that are the same in both stores pass unchanged.
func FromPlay(language string) string {
	if l, ok := fromPlay[language]; ok {
		return l
	}
	return language
}
