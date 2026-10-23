package apple

import (
	"fmt"
	"maps"
	"path/filepath"
	"slices"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/lint"
	"github.com/nikeee/store-cli/internal/media"
	"github.com/nikeee/store-cli/internal/metadata"
)

// Sources of the lint rules, retrieved 2026-10-23:
//   - required fields: "This property is required" for description, keywords
//     and support URL, each keyword "greater than two characters"
//     (https://developer.apple.com/help/app-store-connect/reference/app-information/platform-version-information/);
//   - keywords "separated by commas and no spaces", no words of the name or
//     subtitle, no "app" (https://developer.apple.com/app-store/search/);
//   - iphone-6.5 "Required if app runs on iPhone and screenshots for iPhone
//     with Dynamic Island (large display) aren't provided", ipad-13
//     "Required if app runs on iPad", "You must use the same screenshot size
//     for Apple Watch consistently across all localizations"
//     (https://developer.apple.com/help/app-store-connect/reference/screenshot-specifications).
var requiredFiles = []string{"description.txt", "keywords.txt", "support_url.txt"}

// Lint checks the directories of o offline: every problem push finds is an
// error, plus the lint rules of the files and screenshots. With notesDir it
// checks the release notes in it too. The error is for what stops reading.
func Lint(o PushOptions, notesDir string) (*lint.Result, error) {
	var c clierr.Collector
	in, err := readPushDir(o, &c)
	if err != nil {
		return nil, err
	}
	in.validate(&c)
	r := &lint.Result{Findings: []lint.Finding{}}
	r.AddError(c.Err())
	if notesDir != "" {
		_, err := ReadReleaseNotesDir(notesDir)
		r.AddError(err)
	}
	in.lintTexts(r)
	in.lintScreenshots(r)
	r.Sort()
	return r, nil
}

// appleField returns the App Store field of file.
func appleField(file string) metadata.Field {
	f, _ := metadata.FieldByFile(metadata.AppleFields, file)
	return f
}

func (in *PushInput) lintTexts(r *lint.Result) {
	for _, l := range slices.Sorted(maps.Keys(in.locales)) {
		t := in.locales[l]
		dir := filepath.Join(in.opts.Dir, l)
		for _, file := range requiredFiles {
			if v, ok := t[appleField(file).API]; ok && v == "" {
				r.Add(lint.Error, clierr.Issue{File: filepath.Join(dir, file), Message: "must not be empty, App Store Connect requires it to submit"})
			}
		}
		kw := t[appleField("keywords.txt").API]
		for _, is := range keywordIssues(filepath.Join(dir, "keywords.txt"), kw, t[appleField(nameFile).API], t[appleField("subtitle.txt").API]) {
			r.Add(lint.Warning, is)
		}
	}
}

// keywordIssues returns the warnings for the keywords kw of file, of a locale
// with name and subtitle.
func keywordIssues(file, kw, name, subtitle string) []clierr.Issue {
	var out []clierr.Issue
	at := func(i int, format string, args ...any) {
		line, col := metadata.Position(kw, i)
		out = append(out, clierr.Issue{File: file, Line: line, Col: col, Message: fmt.Sprintf(format, args...)})
	}
	nameWords, subtitleWords := words(name), words(subtitle)
	seen := map[string]bool{}
	spaces, firstSpace := 0, -1
	start := 0 // rune index of raw in kw
	for _, raw := range strings.Split(kw, ",") {
		term := strings.TrimSpace(raw)
		lead := utf8.RuneCountInString(raw) - utf8.RuneCountInString(strings.TrimLeftFunc(raw, unicode.IsSpace))
		if n := len(raw) - len(term); n > 0 {
			if firstSpace < 0 {
				firstSpace = start
				if lead == 0 {
					firstSpace += utf8.RuneCountInString(term)
				}
			}
			spaces += n
		}
		i := start + lead
		start += utf8.RuneCountInString(raw) + 1
		if term == "" {
			continue
		}
		if utf8.RuneCountInString(term) <= 2 {
			at(i, "keyword %q is shorter than 3 characters", term)
		}
		lower := strings.ToLower(term)
		if seen[lower] {
			at(i, "keyword %q appears twice", term)
			continue
		}
		seen[lower] = true
		termWords := words(term)
		if termWords["app"] {
			at(i, "keyword %q contains \"app\", which does not help search", term)
		}
		delete(termWords, "app") // warned about above
		for _, w := range slices.Sorted(maps.Keys(termWords)) {
			if nameWords[w] {
				at(i, "keyword %q repeats %q from the name, which is searchable already", term, w)
				break
			}
			if subtitleWords[w] {
				at(i, "keyword %q repeats %q from the subtitle, which is searchable already", term, w)
				break
			}
		}
	}
	if spaces > 0 {
		at(firstSpace, "spaces next to commas use %d of the %d bytes", spaces, appleField("keywords.txt").MaxBytes)
	}
	return out
}

// words returns the lower case words (runs of letters and digits) of s.
func words(s string) map[string]bool {
	out := map[string]bool{}
	for _, w := range strings.FieldsFunc(strings.ToLower(s), func(r rune) bool { return !unicode.IsLetter(r) && !unicode.IsNumber(r) }) {
		out[w] = true
	}
	return out
}

func (in *PushInput) lintScreenshots(r *lint.Result) {
	if len(in.shots) == 0 {
		return
	}
	has := map[string]bool{} // device alias -> a set in any locale
	var watch []string
	for _, sets := range in.shots {
		for alias, paths := range sets {
			has[alias] = true
			if strings.HasPrefix(alias, "watch-") {
				watch = append(watch, paths...)
			}
		}
	}
	if in.opts.Platform == "ios" && !has["iphone-6.9"] && !has["iphone-6.5"] {
		r.Add(lint.Warning, clierr.Issue{Message: "no iphone-6.9 or iphone-6.5 screenshots; App Store Connect requires one of them if the app runs on iPhone"})
	}
	ipad := slices.ContainsFunc(slices.Collect(maps.Keys(has)), func(a string) bool { return strings.HasPrefix(a, "ipad-") })
	if ipad && !has["ipad-13"] {
		r.Add(lint.Warning, clierr.Issue{Message: "iPad screenshots but no ipad-13 ones; App Store Connect requires ipad-13 if the app runs on iPad"})
	}
	slices.Sort(watch)
	lintWatchSizes(r, watch)
}

// lintWatchSizes reports the Apple Watch screenshots whose size differs from
// the size most of them have.
func lintWatchSizes(r *lint.Result, paths []string) {
	bySize := map[string][]string{}
	var sizes []string // in order of first use
	for _, p := range paths {
		info, err := media.Inspect(p)
		if err != nil {
			continue // validate reported it
		}
		s := fmt.Sprintf("%dx%d", info.Width, info.Height)
		if bySize[s] == nil {
			sizes = append(sizes, s)
		}
		bySize[s] = append(bySize[s], p)
	}
	if len(sizes) < 2 {
		return
	}
	main := sizes[0]
	for _, s := range sizes[1:] {
		if len(bySize[s]) > len(bySize[main]) {
			main = s
		}
	}
	for _, s := range sizes {
		if s == main {
			continue
		}
		for _, p := range bySize[s] {
			r.Add(lint.Error, clierr.Issue{File: p, Message: fmt.Sprintf("%s, the other Apple Watch screenshots are %s; App Store Connect needs one size in all locales", s, main)})
		}
	}
}
