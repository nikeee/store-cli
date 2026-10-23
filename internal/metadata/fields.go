// Package metadata describes every listing text field once and implements
// the offline logic around the fastlane-style metadata directory: reading,
// writing, scaffolding, diffing and validation. It does not talk to any API.
package metadata

import "slices"

// Level says where a field lives in the store's data model.
type Level string

const (
	LevelAppInfo Level = "app-info"
	LevelVersion Level = "version"
	LevelListing Level = "listing"
	LevelReview  Level = "review"
)

// Field describes one text file of the metadata directory.
type Field struct {
	File     string // "description.txt"
	Flag     string // "description"; empty if there is no listing set flag
	API      string // JSON attribute name, "description"
	Level    Level
	MaxChars int // 0 = no limit; counted in runes
	MaxBytes int // 0 = no limit; counted in UTF-8 bytes
	MinChars int // 0 = no minimum; an empty value is not checked
	Help     string
	Secret   bool // value is masked in output (see MaskSecrets)
	Bool     bool // value is "true" or "false"; empty means no change
}

// Texts maps Field.API to the value, only for files that exist.
type Texts map[string]string

// AppleFields are the per-locale App Store text fields.
//
// App Store Connect help, retrieved 2026-10-23: a name has "at least two
// characters" (https://developer.apple.com/help/app-store-connect/reference/app-information/app-information/),
// keywords take "up to 100 bytes of content"
// (https://developer.apple.com/help/app-store-connect/reference/app-information/platform-version-information/).
var AppleFields = []Field{
	{"name.txt", "name", "name", LevelAppInfo, 30, 0, 2, "App name", false, false},
	{"subtitle.txt", "subtitle", "subtitle", LevelAppInfo, 30, 0, 0, "App subtitle", false, false},
	{"privacy_url.txt", "privacy-url", "privacyPolicyUrl", LevelAppInfo, 0, 0, 0, "Privacy policy URL", false, false},
	{"privacy_choices_url.txt", "privacy-choices-url", "privacyChoicesUrl", LevelAppInfo, 0, 0, 0, "Privacy choices URL", false, false},
	{"description.txt", "description", "description", LevelVersion, 4000, 0, 0, "App description", false, false},
	{"keywords.txt", "keywords", "keywords", LevelVersion, 0, 100, 0, "Search keywords, comma separated", false, false},
	{"promotional_text.txt", "promotional-text", "promotionalText", LevelVersion, 170, 0, 0, "Promotional text", false, false},
	{"marketing_url.txt", "marketing-url", "marketingUrl", LevelVersion, 0, 0, 0, "Marketing URL", false, false},
	{"support_url.txt", "support-url", "supportUrl", LevelVersion, 0, 0, 0, "Support URL", false, false},
	{"release_notes.txt", "", "whatsNew", LevelVersion, 4000, 0, 0, "What's New text (set with release-notes set)", false, false},
}

// AppleReviewFields are the files of the review_information/ folder.
var AppleReviewFields = []Field{
	{"first_name.txt", "first-name", "contactFirstName", LevelReview, 0, 0, 0, "Review contact first name", false, false},
	{"last_name.txt", "last-name", "contactLastName", LevelReview, 0, 0, 0, "Review contact last name", false, false},
	{"phone_number.txt", "phone", "contactPhone", LevelReview, 0, 0, 0, "Review contact phone number", false, false},
	{"email_address.txt", "email", "contactEmail", LevelReview, 0, 0, 0, "Review contact email address", false, false},
	{"demo_required.txt", "demo-required", "demoAccountRequired", LevelReview, 0, 0, 0, "Whether a demo account is needed: true or false", false, true},
	{"demo_user.txt", "demo-user", "demoAccountName", LevelReview, 0, 0, 0, "Demo account user name", false, false},
	{"demo_password.txt", "demo-password", "demoAccountPassword", LevelReview, 0, 0, 0, "Demo account password", true, false},
	{"notes.txt", "notes", "notes", LevelReview, 4000, 0, 0, "Notes for the App Review team", false, false},
}

// GoogleFields are the per-locale Google Play listing text fields.
var GoogleFields = []Field{
	{"title.txt", "title", "title", LevelListing, 30, 0, 0, "App title", false, false},
	{"short_description.txt", "short-description", "shortDescription", LevelListing, 80, 0, 0, "Short description", false, false},
	{"full_description.txt", "full-description", "fullDescription", LevelListing, 4000, 0, 0, "Full description", false, false},
	{"video.txt", "video", "video", LevelListing, 0, 0, 0, "Promo video URL", false, false},
}

// FieldByFlag returns the field with the given listing set flag. An empty
// flag never matches.
func FieldByFlag(fields []Field, flag string) (Field, bool) {
	if flag == "" {
		return Field{}, false
	}
	for _, f := range fields {
		if f.Flag == flag {
			return f, true
		}
	}
	return Field{}, false
}

// FieldByFile returns the field stored in the given file.
func FieldByFile(fields []Field, file string) (Field, bool) {
	for _, f := range fields {
		if f.File == file {
			return f, true
		}
	}
	return Field{}, false
}

// Mask replaces a secret value in output.
const Mask = "***"

// MaskSecrets returns a copy of changes with Old and New of secret fields
// replaced by Mask when non-empty.
func MaskSecrets(fields []Field, changes []Change) []Change {
	out := slices.Clone(changes)
	for i, c := range out {
		if f, ok := fieldByAPI(fields, c.Field); !ok || !f.Secret {
			continue
		}
		if c.Old != "" {
			out[i].Old = Mask
		}
		if c.New != "" {
			out[i].New = Mask
		}
	}
	return out
}

func fieldByAPI(fields []Field, api string) (Field, bool) {
	for _, f := range fields {
		if f.API == api {
			return f, true
		}
	}
	return Field{}, false
}
