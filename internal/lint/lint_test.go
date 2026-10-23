package lint

import (
	"bytes"
	"errors"
	"reflect"
	"testing"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/output"
)

func sample() *Result {
	r := &Result{}
	var c clierr.Collector
	c.Add(clierr.At("de-DE/name.txt", clierr.Issue{File: "m/de-DE/name.txt", Line: 1, Col: 31, Message: "31 characters, limit 30"}))
	c.Add(clierr.New("es-MX: Google Play calls this language es-419, rename the folder"))
	r.AddError(c.Err())
	r.Add(Warning, clierr.Issue{File: "m/de-DE/keywords.txt", Line: 1, Col: 4, Message: "keyword \"ab\" is shorter than 3 characters"})
	r.Add(Warning, clierr.Issue{Message: "no iphone-6.9 or iphone-6.5 screenshots"})
	r.Add(Error, clierr.Issue{File: "m/de-DE/description.txt", Message: "must not be empty"})
	r.Sort()
	return r
}

func TestResultCollectsAndSorts(t *testing.T) {
	r := sample()
	want := []Finding{
		{Severity: Error, Message: "es-MX: Google Play calls this language es-419, rename the folder"},
		{Severity: Warning, Message: "no iphone-6.9 or iphone-6.5 screenshots"},
		{Severity: Error, File: "m/de-DE/description.txt", Message: "must not be empty"},
		{Severity: Warning, File: "m/de-DE/keywords.txt", Line: 1, Col: 4, Message: "keyword \"ab\" is shorter than 3 characters"},
		{Severity: Error, File: "m/de-DE/name.txt", Line: 1, Col: 31, Message: "31 characters, limit 30"},
	}
	if !reflect.DeepEqual(r.Findings, want) || r.Errors != 3 || r.Warnings != 2 {
		t.Errorf("got %+v, %d errors, %d warnings", r.Findings, r.Errors, r.Warnings)
	}
	r.AddError(nil)
	r.AddError(errors.New("plain"))
	if last := r.Findings[len(r.Findings)-1]; last != (Finding{Severity: Error, Message: "plain"}) || r.Errors != 4 {
		t.Errorf("plain error: %+v", last)
	}
}

func TestResultErr(t *testing.T) {
	r := sample()
	err := r.Err(false)
	var e *clierr.Error
	if !errors.As(err, &e) || e.Message != "3 errors, 2 warnings" || clierr.ExitCode(err) != 1 {
		t.Fatalf("err = %v", err)
	}
	wantIssues := []clierr.Issue{
		{Message: "es-MX: Google Play calls this language es-419, rename the folder"},
		{File: "m/de-DE/description.txt", Message: "must not be empty"},
		{File: "m/de-DE/name.txt", Line: 1, Col: 31, Message: "31 characters, limit 30"},
	}
	if !reflect.DeepEqual(e.Issues, wantIssues) {
		t.Errorf("issues = %+v", e.Issues)
	}

	warn := &Result{}
	warn.Add(Warning, clierr.Issue{Message: "w"})
	if err := warn.Err(false); err != nil {
		t.Errorf("warnings only: %v", err)
	}
	err = warn.Err(true)
	if !errors.As(err, &e) || e.Message != "0 errors, 1 warning (--strict)" || len(e.Issues) != 0 {
		t.Errorf("strict: %v", err)
	}
	if err := (&Result{}).Err(true); err != nil {
		t.Errorf("clean: %v", err)
	}
}

func TestRenderText(t *testing.T) {
	var b bytes.Buffer
	if err := sample().RenderText(&b, output.NewStyles(false)); err != nil {
		t.Fatal(err)
	}
	want := "error: es-MX: Google Play calls this language es-419, rename the folder\n" +
		"warning: no iphone-6.9 or iphone-6.5 screenshots\n" +
		"m/de-DE/description.txt: error: must not be empty\n" +
		"m/de-DE/keywords.txt:1:4: warning: keyword \"ab\" is shorter than 3 characters\n" +
		"m/de-DE/name.txt:1:31: error: 31 characters, limit 30\n" +
		"3 errors, 2 warnings\n"
	if b.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", b.String(), want)
	}
	b.Reset()
	if err := (&Result{}).RenderText(&b, output.NewStyles(false)); err != nil || b.String() != "no problems\n" {
		t.Errorf("clean: %q, %v", b.String(), err)
	}
}
