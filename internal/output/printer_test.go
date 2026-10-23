package output_test

import (
	"bytes"
	"encoding/json"
	"io"
	"strings"
	"testing"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/output"
	"github.com/nikeee/store-cli/internal/testutil/golden"
)

type item struct {
	A string `json:"a"`
	B string `json:"b"`
}

type sample struct {
	ID    string   `json:"id"`
	Name  string   `json:"name"`
	Count int      `json:"count"`
	Tags  []string `json:"tags"`
	Items []item   `json:"items"`
}

var sampleOne = sample{
	ID: "app-1", Name: "My App", Count: 3, Tags: []string{"a", "b"},
	Items: []item{{"x1", "y1"}, {"x2", "y2"}},
}

var sampleMany = []sample{sampleOne, {ID: "app-2", Name: "Other, App", Count: 0}}

func render(t *testing.T, mode output.Mode, v any) []byte {
	t.Helper()
	var out, errb bytes.Buffer
	if err := output.NewPrinter(mode, &out, &errb, false).Result(v); err != nil {
		t.Fatal(err)
	}
	if errb.Len() != 0 {
		t.Fatalf("stderr: %q", errb.String())
	}
	return out.Bytes()
}

func TestPrinterGolden(t *testing.T) {
	for _, mode := range []output.Mode{output.ModeText, output.ModeJSON, output.ModeTOON} {
		t.Run(string(mode)+"/one", func(t *testing.T) {
			golden.Assert(t, "one."+string(mode), render(t, mode, sampleOne))
		})
		t.Run(string(mode)+"/many", func(t *testing.T) {
			golden.Assert(t, "many."+string(mode), render(t, mode, sampleMany))
		})
	}
}

func TestPrinterEmptyList(t *testing.T) {
	if got := string(render(t, output.ModeText, []sample{})); got != "(none)\n" {
		t.Fatalf("got %q", got)
	}
	if got := string(render(t, output.ModeText, []sample(nil))); got != "(none)\n" {
		t.Fatalf("got %q", got)
	}
}

type custom struct{}

func (custom) RenderText(w io.Writer, s output.Styles) error {
	_, err := io.WriteString(w, s.Key.Render("custom")+"\n")
	return err
}

func TestPrinterTextRenderer(t *testing.T) {
	if got := string(render(t, output.ModeText, custom{})); got != "custom\n" {
		t.Fatalf("got %q", got)
	}
}

func TestPrinterNoColorWhenNotTTY(t *testing.T) {
	// color=false is what cli passes for non-TTY output
	if bytes.Contains(render(t, output.ModeText, sampleMany), []byte("\x1b[")) {
		t.Fatal("escape sequence in output")
	}
}

func TestPrinterColor(t *testing.T) {
	var out, errb bytes.Buffer
	p := output.NewPrinter(output.ModeText, &out, &errb, true)
	if err := p.Result(sampleOne); err != nil {
		t.Fatal(err)
	}
	p.Warnf("careful %d", 1)
	p.Progressf("step")
	p.Error(clierr.New("boom"))
	if !strings.Contains(out.String(), "\x1b[") || !strings.Contains(errb.String(), "\x1b[") {
		t.Fatalf("expected colors: %q %q", out.String(), errb.String())
	}
}

// json mode never colors, even on a terminal. NO_COLOR is applied by cli,
// see TestRunColor there.
func TestPrinterJSONNeverColors(t *testing.T) {
	var out, errb bytes.Buffer
	p := output.NewPrinter(output.ModeJSON, &out, &errb, true)
	p.Warnf("w")
	_ = p.Result(sampleOne)
	if strings.Contains(out.String()+errb.String(), "\x1b[") {
		t.Fatal("json mode must not color")
	}
}

func TestPrinterErrorJSON(t *testing.T) {
	var out, errb bytes.Buffer
	p := output.NewPrinter(output.ModeJSON, &out, &errb, false)
	p.Error(&clierr.Error{Message: "x", Status: 409, Code: "STATE_ERROR", Hint: "store apple version cancel 1.0"})
	if out.Len() != 0 {
		t.Fatalf("stdout: %q", out.String())
	}
	var got map[string]any
	if err := json.Unmarshal(errb.Bytes(), &got); err != nil {
		t.Fatal(err)
	}
	want := map[string]any{"error": "x", "status": 409.0, "code": "STATE_ERROR", "hint": "store apple version cancel 1.0"}
	if len(got) != len(want) {
		t.Fatalf("got %v", got)
	}
	for k, v := range want {
		if got[k] != v {
			t.Errorf("%s: got %v want %v", k, got[k], v)
		}
	}
}

func TestPrinterErrorTextAndTOON(t *testing.T) {
	e := &clierr.Error{Message: "x", Detail: "line1\nline2", Hint: "store version"}
	var out, errb bytes.Buffer
	output.NewPrinter(output.ModeText, &out, &errb, false).Error(e)
	golden.Assert(t, "error.text", errb.Bytes())
	errb.Reset()
	output.NewPrinter(output.ModeTOON, &out, &errb, false).Error(e)
	golden.Assert(t, "error.toon", errb.Bytes())
	errb.Reset()
	output.NewPrinter(output.ModeText, &out, &errb, false).Error(errPlain("plain"))
	if errb.String() != "error: plain\n" {
		t.Fatalf("got %q", errb.String())
	}
}

type errPlain string

func (e errPlain) Error() string { return string(e) }

func TestReporter(t *testing.T) {
	var out, errb bytes.Buffer
	var r output.Reporter = output.NewPrinter(output.ModeJSON, &out, &errb, false)
	r.Progressf("uploading %d", 1)
	r.Warnf("slow")
	if errb.String() != "uploading 1\nwarning: slow\n" || out.Len() != 0 {
		t.Fatalf("out=%q err=%q", out.String(), errb.String())
	}
}

// stdout and stderr get colors each by their own terminal check.
func TestPrinterColorsStreamsSeparately(t *testing.T) {
	var out, errb bytes.Buffer
	p := output.NewPrinter(output.ModeText, &out, &errb, false)
	p.ColorStderr(true)
	p.Warnf("careful")
	if err := p.Result(map[string]string{"a": "b"}); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(errb.String(), "\x1b[") || strings.Contains(out.String(), "\x1b[") {
		t.Errorf("stdout %q, stderr %q", out.String(), errb.String())
	}
}
