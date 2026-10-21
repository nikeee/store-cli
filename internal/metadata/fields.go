// Package metadata describes every listing text field once and implements
// the offline logic around the fastlane-style metadata directory: reading,
// writing, scaffolding, diffing and validation. It does not talk to any API.
package metadata

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
	Help     string
}

// Texts maps Field.API to the value, only for files that exist.
type Texts map[string]string

// AppleFields are the per-locale App Store text fields.
var AppleFields = []Field{
	{"name.txt", "name", "name", LevelAppInfo, 30, "App name"},
	{"subtitle.txt", "subtitle", "subtitle", LevelAppInfo, 30, "App subtitle"},
	{"privacy_url.txt", "privacy-url", "privacyPolicyUrl", LevelAppInfo, 0, "Privacy policy URL"},
	{"privacy_choices_url.txt", "privacy-choices-url", "privacyChoicesUrl", LevelAppInfo, 0, "Privacy choices URL"},
	{"description.txt", "description", "description", LevelVersion, 4000, "App description"},
	{"keywords.txt", "keywords", "keywords", LevelVersion, 100, "Search keywords, comma separated"},
	{"promotional_text.txt", "promotional-text", "promotionalText", LevelVersion, 170, "Promotional text"},
	{"marketing_url.txt", "marketing-url", "marketingUrl", LevelVersion, 0, "Marketing URL"},
	{"support_url.txt", "support-url", "supportUrl", LevelVersion, 0, "Support URL"},
	{"release_notes.txt", "", "whatsNew", LevelVersion, 4000, "What's New text (set with release-notes set)"},
}

// AppleReviewFields are the files of the review_information/ folder.
var AppleReviewFields = []Field{
	{"first_name.txt", "first-name", "contactFirstName", LevelReview, 0, "Review contact first name"},
	{"last_name.txt", "last-name", "contactLastName", LevelReview, 0, "Review contact last name"},
	{"phone_number.txt", "phone", "contactPhone", LevelReview, 0, "Review contact phone number"},
	{"email_address.txt", "email", "contactEmail", LevelReview, 0, "Review contact email address"},
	{"demo_required.txt", "demo-required", "demoAccountRequired", LevelReview, 0, "Whether a demo account is needed: true or false"},
	{"demo_user.txt", "demo-user", "demoAccountName", LevelReview, 0, "Demo account user name"},
	{"demo_password.txt", "demo-password", "demoAccountPassword", LevelReview, 0, "Demo account password"},
	{"notes.txt", "notes", "notes", LevelReview, 4000, "Notes for the App Review team"},
}

// GoogleFields are the per-locale Google Play listing text fields.
var GoogleFields = []Field{
	{"title.txt", "title", "title", LevelListing, 30, "App title"},
	{"short_description.txt", "short-description", "shortDescription", LevelListing, 80, "Short description"},
	{"full_description.txt", "full-description", "fullDescription", LevelListing, 4000, "Full description"},
	{"video.txt", "video", "video", LevelListing, 0, "Promo video URL"},
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
