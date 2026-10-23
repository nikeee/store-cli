package google

import (
	"fmt"
	"maps"
	"path/filepath"
	"regexp"
	"slices"
	"unicode"
	"unicode/utf8"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/lint"
	"github.com/nikeee/store-cli/internal/media"
	"github.com/nikeee/store-cli/internal/metadata"
)

// Sources of the lint rules, retrieved 2026-10-23:
//   - "You must provide a feature graphic to publish", "a minimum of two
//     screenshots across different device types", and to be eligible for
//     promotion "at least four screenshots at 1080 px minimum", 16:9 or 9:16
//     (https://support.google.com/googleplay/android-developer/answer/9866151);
//   - in the title "Don't use emojis, emoticons, or repeated special
//     characters", "Avoid ALL CAPS unless it is part of your brand name", no
//     text that indicates "store performance or ranking" or promotions
//     (https://support.google.com/googleplay/android-developer/answer/9898842).
const (
	promoShots   = 4    // phone screenshots for promotion
	promoMinSide = 1080 // px, short side of a screenshot for promotion
	minAllCaps   = 4    // fewer capital letters may be an acronym like "TV"
)

var (
	emoticonRe = regexp.MustCompile(`[:;]-?[)(]|[:;]-?[DPp]\b|<3\b`)
	promoRe    = regexp.MustCompile(`(?i)#1\b|\b(?:best|free)\b|%\s*off\b`)
)

// Lint checks <dir>/android/ of o offline: every problem push finds is an
// error, plus the lint rules of the titles and images. With notesDir it
// checks the release notes in it too. The error is for what stops reading.
func Lint(o PushOptions, notesDir string) (*lint.Result, error) {
	in, err := readPushDir(o)
	if err != nil {
		return nil, err
	}
	var c clierr.Collector
	in.validate(&c)
	r := &lint.Result{}
	r.AddError(c.Err())
	if notesDir != "" {
		notes, err := ReadReleaseNotesDir(notesDir)
		if err == nil {
			err = ValidateReleaseNotes(notes)
		}
		r.AddError(err)
	}
	in.lintTitles(r)
	in.lintImages(r)
	r.Sort()
	return r, nil
}

func (in *PushInput) lintTitles(r *lint.Result) {
	title, _ := metadata.FieldByFile(metadata.GoogleFields, "title.txt")
	for _, l := range slices.Sorted(maps.Keys(in.locales)) {
		file := filepath.Join(in.opts.Dir, metadata.AndroidDir, l, title.File)
		for _, is := range titleIssues(file, in.locales[l][title.API]) {
			r.Add(lint.Warning, is)
		}
	}
}

// titleIssues returns the warnings for the title of file.
func titleIssues(file, title string) []clierr.Issue {
	var out []clierr.Issue
	at := func(byteIdx int, format string, args ...any) {
		line, col := metadata.Position(title, utf8.RuneCountInString(title[:byteIdx]))
		out = append(out, clierr.Issue{File: file, Line: line, Col: col, Message: fmt.Sprintf(format, args...)})
	}
	if i := emojiIndex(title); i >= 0 {
		r, _ := utf8.DecodeRuneInString(title[i:])
		at(i, "emoji or emoticon %q; Google Play does not allow them in the title", string(r))
	} else if m := emoticonRe.FindStringIndex(title); m != nil {
		at(m[0], "emoji or emoticon %q; Google Play does not allow them in the title", title[m[0]:m[1]])
	}
	upper, lower := 0, 0
	for _, r := range title {
		switch {
		case unicode.IsUpper(r):
			upper++
		case unicode.IsLower(r):
			lower++
		}
	}
	if upper >= minAllCaps && lower == 0 {
		out = append(out, clierr.Issue{File: file, Line: 1, Message: "all capital letters; Google Play allows them only in brand names"})
	}
	var prev rune
	for i, r := range title {
		// "..." is common and fine
		if r == prev && r != '.' && (unicode.IsPunct(r) || unicode.IsSymbol(r)) && !isEmoji(r) {
			at(i-utf8.RuneLen(r), "repeated %q; Google Play does not allow repeated special characters in the title", string(r))
			break
		}
		prev = r
	}
	if m := promoRe.FindStringIndex(title); m != nil {
		at(m[0], "%q suggests ranking or a promotion; Google Play does not allow it in the title", title[m[0]:m[1]])
	}
	return out
}

// emojiIndex returns the byte index of the first emoji in s, or -1.
func emojiIndex(s string) int {
	for i, r := range s {
		if isEmoji(r) {
			return i
		}
	}
	return -1
}

// isEmoji reports whether r is in a block of emoji and pictographs.
func isEmoji(r rune) bool {
	return r >= 0x1F000 && r <= 0x1FAFF || r >= 0x2600 && r <= 0x27BF || r >= 0x2B00 && r <= 0x2BFF || r == 0xFE0F
}

func (in *PushInput) lintImages(r *lint.Result) {
	if len(in.images) == 0 {
		return
	}
	feature, mostShots := false, 0
	for _, l := range slices.Sorted(maps.Keys(in.images)) {
		sets := in.images[l]
		n := 0
		for _, t := range media.ImageTypes {
			if t.Multiple { // the screenshot types
				n += len(sets[t.Alias])
			}
		}
		mostShots = max(mostShots, n)
		feature = feature || len(sets["feature-graphic"]) > 0
		phone := sets["phone"]
		// fewer than 2 is an error of push already
		if len(phone) >= 2 && len(phone) < promoShots {
			r.Add(lint.Warning, clierr.Issue{Message: fmt.Sprintf("%s: only %d phone screenshots; Google Play promotes apps with at least %d", l, len(phone), promoShots)})
		}
		for _, p := range phone {
			lintPromoSize(r, p)
		}
	}
	if !feature {
		r.Add(lint.Warning, clierr.Issue{Message: "no feature graphic; Google Play requires one to publish"})
	}
	if mostShots < 2 {
		r.Add(lint.Warning, clierr.Issue{Message: "no locale has 2 screenshots; Google Play requires at least 2 to publish"})
	}
}

// lintPromoSize warns about a phone screenshot that is not 9:16 or 16:9 with
// a short side of at least 1080 px.
func lintPromoSize(r *lint.Result, path string) {
	info, err := media.Inspect(path)
	if err != nil {
		return // validate reported it
	}
	short, long := min(info.Width, info.Height), max(info.Width, info.Height)
	// 16:9 within about a pixel
	if d := long*9 - short*16; short >= promoMinSide && d >= -16 && d <= 16 {
		return
	}
	r.Add(lint.Warning, clierr.Issue{File: path, Message: fmt.Sprintf("%dx%d; Google Play promotes apps with 9:16 or 16:9 screenshots of at least %d px", info.Width, info.Height, promoMinSide)})
}
