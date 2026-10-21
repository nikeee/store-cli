package metadata

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
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
	err := ValidateLocaleTexts("de-DE", AppleFields, Texts{"subtitle": strings.Repeat("ä", 31)})
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
	got, err := ReadReleaseNotesDir(dir)
	if err != nil {
		t.Fatal(err)
	}
	if want := map[string]string{"en-US": "Fixes", "de-DE": "Fehler"}; !reflect.DeepEqual(got, want) {
		t.Errorf("got %v", got)
	}

	bad := t.TempDir()
	write(t, filepath.Join(bad, "notes.txt"), "x")
	if _, err := ReadReleaseNotesDir(bad); err == nil || !strings.Contains(err.Error(), "notes.txt") {
		t.Errorf("invalid locale: %v", err)
	}
	empty := t.TempDir()
	write(t, filepath.Join(empty, "de-DE.txt"), "\n")
	if _, err := ReadReleaseNotesDir(empty); err == nil || !strings.Contains(err.Error(), "de-DE.txt") {
		t.Errorf("empty: %v", err)
	}
}

func TestMaskSecrets(t *testing.T) {
	in := []Change{
		{Field: "demoAccountPassword", Old: "old", New: "new"},
		{Field: "demoAccountPassword", Old: "", New: ""},
		{Field: "notes", Old: "a", New: "b"},
	}
	got := MaskSecrets(AppleReviewFields, in)
	if got[0].Old != "***" || got[0].New != "***" || got[1].Old != "" || got[1].New != "" || got[2].Old != "a" || got[2].New != "b" {
		t.Errorf("got = %+v", got)
	}
	if in[0].New != "new" {
		t.Error("input modified")
	}
}
