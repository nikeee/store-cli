package output_test

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/output"
)

func TestAnnotateGitHubIssues(t *testing.T) {
	ws, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	var out, errb bytes.Buffer
	p := output.NewPrinter(output.ModeText, &out, &errb, false)
	p.AnnotateGitHub(ws)
	p.Error(&clierr.Error{Message: "two problems", Issues: []clierr.Issue{
		{File: filepath.Join("meta", "de-DE", "subtitle.txt"), Line: 2, Col: 20, Message: "36 characters, limit 30"},
		{File: filepath.Join(ws, "shots", "a,b:c.png"), Message: "has an alpha channel"},
		{File: "/elsewhere/x.txt", Line: 1, Message: "100% wrong\nreally"},
		{Message: "en-US: phone needs at least 2 screenshots"},
	}})
	want := "::error file=meta/de-DE/subtitle.txt,line=2,col=20::36 characters, limit 30\n" +
		"::error file=shots/a%2Cb%3Ac.png::has an alpha channel\n" +
		"::error file=/elsewhere/x.txt,line=1::100%25 wrong%0Areally\n" +
		"::error::en-US: phone needs at least 2 screenshots\n" +
		"error: two problems\n"
	if errb.String() != want || out.Len() != 0 {
		t.Errorf("stderr:\n%s\nwant:\n%s", errb.String(), want)
	}
}

// An error without issues is one annotation with detail and hint; json mode
// keeps its object after the annotations.
func TestAnnotateGitHubWholeError(t *testing.T) {
	var out, errb bytes.Buffer
	p := output.NewPrinter(output.ModeJSON, &out, &errb, false)
	p.AnnotateGitHub("")
	p.Error(&clierr.Error{Message: "version 1.2.0 is in review", Detail: "line 1\nline 2", Hint: "store apple version cancel 1.2.0"})
	first, rest, _ := strings.Cut(errb.String(), "\n")
	if want := "::error::version 1.2.0 is in review%0Aline 1%0Aline 2%0Ahint: store apple version cancel 1.2.0"; first != want {
		t.Errorf("annotation = %q", first)
	}
	var body struct {
		Error string `json:"error"`
	}
	if json.Unmarshal([]byte(rest), &body) != nil || body.Error != "version 1.2.0 is in review" {
		t.Errorf("json body = %q", rest)
	}

	errb.Reset()
	p.Error(errors.New("plain"))
	if !strings.HasPrefix(errb.String(), "::error::plain\n") {
		t.Errorf("plain error: %q", errb.String())
	}
}

func TestAnnotateGitHubWarning(t *testing.T) {
	var errb bytes.Buffer
	p := output.NewPrinter(output.ModeText, io.Discard, &errb, false)
	p.AnnotateGitHub("")
	p.Warnf("release notes %s dropped", "fr-FR")
	if want := "warning: release notes fr-FR dropped\n::warning::release notes fr-FR dropped\n"; errb.String() != want {
		t.Errorf("stderr = %q", errb.String())
	}
}

func TestNoAnnotationsByDefault(t *testing.T) {
	var errb bytes.Buffer
	p := output.NewPrinter(output.ModeText, io.Discard, &errb, false)
	p.Warnf("w")
	p.Error(clierr.Filef("a.png", "has an alpha channel"))
	if strings.Contains(errb.String(), "::") {
		t.Errorf("stderr = %q", errb.String())
	}
}

// AnnotateWarning writes nothing off GitHub Actions.
func TestAnnotateWarning(t *testing.T) {
	var out, errb bytes.Buffer
	p := output.NewPrinter(output.ModeText, &out, &errb, false)
	is := clierr.Issue{File: filepath.Join("meta", "keywords.txt"), Line: 1, Col: 4, Message: "short keyword"}
	p.AnnotateWarning(is)
	if errb.Len() != 0 {
		t.Fatalf("off GitHub: %q", errb.String())
	}
	p.AnnotateGitHub("")
	p.AnnotateWarning(is)
	if want := "::warning file=meta/keywords.txt,line=1,col=4::short keyword\n"; errb.String() != want || out.Len() != 0 {
		t.Errorf("stderr = %q", errb.String())
	}
}
