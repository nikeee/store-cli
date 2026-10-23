package metadata

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/clierr"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestValidateTextsCountsRunes(t *testing.T) {
	if err := ValidateTexts(AppleFields, Texts{"name": strings.Repeat("ä", 30)}); err != nil {
		t.Fatalf("30 runes: %v", err)
	}
	err := ValidateLocaleTexts("de-DE", "", AppleFields, Texts{"subtitle": strings.Repeat("ä", 31)})
	if err == nil || err.Error() != "de-DE/subtitle.txt: 31 characters, limit 30" {
		t.Fatalf("31 runes: %v", err)
	}
	if err := ValidateTexts(AppleFields, Texts{"name": "👍"}); err != nil {
		t.Fatalf("emoji: %v", err)
	}
	err = ValidateTexts(AppleFields, Texts{"name": strings.Repeat("a", 31), "keywords": strings.Repeat("k", 101)})
	if err == nil || !strings.Contains(err.Error(), "name.txt: 31 characters, limit 30") ||
		!strings.Contains(err.Error(), "keywords.txt: 101 characters, limit 100") {
		t.Fatalf("two violations: %v", err)
	}
	if err := ValidateTexts(AppleReviewFields, Texts{"demoAccountRequired": "yes"}); err == nil {
		t.Fatal("demo_required yes accepted")
	}
	for _, v := range []string{"true", "false", ""} {
		if err := ValidateTexts(AppleReviewFields, Texts{"demoAccountRequired": v}); err != nil {
			t.Fatalf("demoAccountRequired %q: %v", v, err)
		}
	}
}

func TestReadAppleTexts(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "en-US/name.txt"), "App\n")
	write(t, filepath.Join(dir, "en-US/subtitle.txt"), "")
	write(t, filepath.Join(dir, "review_information/notes.txt"), "Hello\n")
	write(t, filepath.Join(dir, "android/en-US/title.txt"), "X")
	write(t, filepath.Join(dir, ".git/config"), "x")
	locales, review, err := ReadAppleTexts(dir)
	if err != nil {
		t.Fatal(err)
	}
	want := map[string]Texts{"en-US": {"name": "App", "subtitle": ""}}
	if !reflect.DeepEqual(locales, want) {
		t.Errorf("locales = %v", locales)
	}
	if !reflect.DeepEqual(review, Texts{"notes": "Hello"}) {
		t.Errorf("review = %v", review)
	}
}

func TestReadGoogleTexts(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "android/de-DE/title.txt"), "Titel\n")
	write(t, filepath.Join(dir, "en-US/name.txt"), "App")
	got, err := ReadGoogleTexts(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]Texts{"de-DE": {"title": "Titel"}}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v", got)
	}
}

func TestWriteTextsThenReadRoundTrip(t *testing.T) {
	dir := t.TempDir()
	in := Texts{"name": "App", "keywords": "", "description": "a\nb"}
	written, err := WriteTexts(filepath.Join(dir, "fr-FR"), AppleFields, in)
	if err != nil {
		t.Fatal(err)
	}
	if len(written) != 3 {
		t.Errorf("written = %v", written)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "fr-FR/name.txt"))
	if string(b) != "App\n" {
		t.Errorf("content %q", b)
	}
	locales, _, err := ReadAppleTexts(dir)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(locales["fr-FR"], in) {
		t.Errorf("round trip = %v", locales)
	}
}

func TestScaffoldNeverOverwrites(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "en-US/name.txt"), "Keep")
	res, err := Scaffold(dir, GoogleFields, []string{"en-US", "de-DE"})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Existing) != 0 || len(res.Created) != 8 {
		t.Fatalf("google: %+v", res)
	}
	res, err = Scaffold(dir, AppleFields, []string{"en-US"})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(res.Existing, []string{filepath.Join(dir, "en-US/name.txt")}) || len(res.Created) != len(AppleFields)-1 {
		t.Fatalf("apple: %+v", res)
	}
	b, _ := os.ReadFile(filepath.Join(dir, "en-US/name.txt"))
	if string(b) != "Keep" {
		t.Errorf("overwritten: %q", b)
	}
	var all InitResult
	all.Merge(res)
	all.Merge(res)
	if len(all.Existing) != 2 {
		t.Errorf("merge: %+v", all)
	}
	r, err := ScaffoldDir(filepath.Join(dir, "review_information"), AppleReviewFields)
	if err != nil || len(r.Created) != len(AppleReviewFields) {
		t.Errorf("scaffold dir: %+v %v", r, err)
	}
}

func TestDiffTexts(t *testing.T) {
	local := Texts{"name": "New", "description": "d", "subtitle": "s"}
	if got := DiffTexts(LevelAppInfo, "en-US", AppleFields, Texts{"name": "A"}, Texts{"name": "A"}); len(got) != 0 {
		t.Errorf("equal: %v", got)
	}
	got := DiffTexts(LevelAppInfo, "en-US", AppleFields, local, Texts{"name": "Old", "subtitle": "s"})
	want := []Change{{Action: ActionUpdate, Scope: "app-info", Locale: "en-US", Field: "name", Old: "Old", New: "New"}}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("update: %v", got)
	}
	got = DiffTexts(LevelAppInfo, "en-US", AppleFields, local, nil)
	if len(got) != 2 || got[0].Action != ActionCreate || got[0].Field != "name" || got[1].Field != "subtitle" {
		t.Errorf("create: %v", got)
	}
}

func TestFieldByFlag(t *testing.T) {
	if f, ok := FieldByFlag(AppleFields, "keywords"); !ok || f.File != "keywords.txt" {
		t.Errorf("keywords: %v %v", f, ok)
	}
	if _, ok := FieldByFlag(AppleFields, ""); ok {
		t.Error("empty flag matched")
	}
}

func TestReadReleaseNotesDir(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "en-US.txt"), "Fixes\n")
	write(t, filepath.Join(dir, "de-DE.txt"), "Fehler")
	write(t, filepath.Join(dir, "README.md"), "ignored")
	got, err := ReadReleaseNotesDir(dir, 500)
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"en-US": "Fixes", "de-DE": "Fehler"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v", got)
	}

	bad := t.TempDir()
	write(t, filepath.Join(bad, "notes.txt"), "x")
	if _, err := ReadReleaseNotesDir(bad, 500); err == nil || !strings.Contains(err.Error(), "notes.txt") {
		t.Errorf("invalid locale: %v", err)
	}
	empty := t.TempDir()
	write(t, filepath.Join(empty, "de-DE.txt"), "\n")
	if _, err := ReadReleaseNotesDir(empty, 500); err == nil || !strings.Contains(err.Error(), "de-DE.txt") {
		t.Errorf("empty: %v", err)
	}
}

// The true/false rule follows the Bool mark of the field, not a file name.
func TestValidateBoolField(t *testing.T) {
	fields := []Field{{File: "flag.txt", API: "flag", Bool: true}, {File: "text.txt", API: "text"}}
	err := ValidateLocaleTexts("en-US", "", fields, Texts{"flag": "yes", "text": "yes"})
	if err == nil || err.Error() != "en-US/flag.txt: must be true or false" {
		t.Fatalf("err = %v", err)
	}
	if f, ok := FieldByFile(AppleReviewFields, "demo_required.txt"); !ok || !f.Bool {
		t.Errorf("demo_required.txt is not a Bool field: %+v", f)
	}
}

func TestMaskSecrets(t *testing.T) {
	in := []Change{
		{Field: "demoAccountPassword", Old: "old", New: "new"},
		{Field: "demoAccountPassword", Old: "", New: ""},
		{Field: "notes", Old: "a", New: "b"},
	}
	got := MaskSecrets(AppleReviewFields, in)
	if got[0].Old != Mask || got[0].New != Mask || got[1].Old != "" || got[1].New != "" || got[2].Old != "a" || got[2].New != "b" {
		t.Errorf("got = %+v", got)
	}
	if in[0].New != "new" {
		t.Error("input modified")
	}
}

func TestReadReleaseNotesDirNoFiles(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "readme.md"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}
	for _, d := range []string{t.TempDir(), dir} {
		if _, err := ReadReleaseNotesDir(d, 500); err == nil || !strings.Contains(err.Error(), "no release notes (<locale>.txt files) in "+d) {
			t.Errorf("%s: err = %v", d, err)
		}
	}
}

// The *PathError of a failed file operation already names the path; the
// message must not repeat it.
func TestFileErrorsNameThePathOnce(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "missing")
	_, err := ReadReleaseNotesDir(missing, 500)
	if err == nil || strings.Count(err.Error(), missing) != 1 {
		t.Errorf("release notes: err = %v", err)
	}
	file := filepath.Join(t.TempDir(), "file")
	if err := os.WriteFile(file, nil, 0o644); err != nil {
		t.Fatal(err)
	}
	_, err = WriteTexts(filepath.Join(file, "sub"), AppleFields, Texts{"name": "x"})
	if err == nil || strings.Count(err.Error(), file) != 1 {
		t.Errorf("write: err = %v", err)
	}
}

func issues(t *testing.T, err error) []clierr.Issue {
	t.Helper()
	var e *clierr.Error
	if !errors.As(err, &e) {
		t.Fatalf("not a *clierr.Error: %v", err)
	}
	return e.Issues
}

func TestPosition(t *testing.T) {
	cases := []struct {
		s         string
		i         int
		line, col int
	}{
		{"abc", 0, 1, 1},
		{"abc", 2, 1, 3},
		{"ab\ncd", 2, 1, 3}, // the newline itself
		{"ab\ncd", 3, 2, 1},
		{"äö\r\nüß", 5, 2, 2},
		{"a\n\nb", 3, 3, 1},
	}
	for _, c := range cases {
		if line, col := position(c.s, c.i); line != c.line || col != c.col {
			t.Errorf("position(%q, %d) = %d:%d, want %d:%d", c.s, c.i, line, col, c.line, c.col)
		}
	}
}

// Violations of texts read from a folder point at the first rune over the
// limit in the file; the message keeps the short name.
func TestValidateLocaleTextsIssues(t *testing.T) {
	dir := filepath.Join("meta", "de-DE")
	texts := Texts{"subtitle": "Zeile eins\n" + strings.Repeat("ü", 25), "name": "App"}
	err := ValidateLocaleTexts("de-DE", dir, AppleFields, texts)
	if err == nil || err.Error() != "de-DE/subtitle.txt: 36 characters, limit 30" {
		t.Fatalf("err = %v", err)
	}
	want := []clierr.Issue{{File: filepath.Join(dir, "subtitle.txt"), Line: 2, Col: 20, Message: "36 characters, limit 30"}}
	if got := issues(t, err); !reflect.DeepEqual(got, want) {
		t.Errorf("issues = %+v", got)
	}
	err = ValidateLocaleTexts("review_information", "review_information", AppleReviewFields, Texts{"demoAccountRequired": "yes"})
	want = []clierr.Issue{{File: filepath.Join("review_information", "demo_required.txt"), Line: 1, Message: "must be true or false"}}
	if got := issues(t, err); !reflect.DeepEqual(got, want) {
		t.Errorf("bool issues = %+v", got)
	}
	// texts not from files have no file to point at
	err = ValidateLocaleTexts("de-DE", "", AppleFields, texts)
	want = []clierr.Issue{{Message: "de-DE/subtitle.txt: 36 characters, limit 30"}}
	if got := issues(t, err); !reflect.DeepEqual(got, want) {
		t.Errorf("issues without dir = %+v", got)
	}
}

func TestReadReleaseNotesDirReportsAllFiles(t *testing.T) {
	dir := t.TempDir()
	write(t, filepath.Join(dir, "de-DE.txt"), "Neu:\n"+strings.Repeat("x", 10)+"\n")
	write(t, filepath.Join(dir, "en-US.txt"), " \n")
	write(t, filepath.Join(dir, "fr-FR.txt"), "ok")
	write(t, filepath.Join(dir, "notes.txt"), "x")
	_, err := ReadReleaseNotesDir(dir, 8)
	de, en, notes := filepath.Join(dir, "de-DE.txt"), filepath.Join(dir, "en-US.txt"), filepath.Join(dir, "notes.txt")
	want := de + ": 15 characters, limit 8; " + en + ": release notes are empty; " + notes + `: "notes" is not a locale name`
	if err == nil || err.Error() != want || clierr.ExitCode(err) != 1 {
		t.Fatalf("err = %v", err)
	}
	wantIssues := []clierr.Issue{
		{File: de, Line: 2, Col: 4, Message: "15 characters, limit 8"},
		{File: en, Message: "release notes are empty"},
		{File: notes, Message: `"notes" is not a locale name`},
	}
	if got := issues(t, err); !reflect.DeepEqual(got, wantIssues) {
		t.Errorf("issues = %+v", got)
	}
}
