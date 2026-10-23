package output

import (
	"strings"
	"testing"
)

type inner struct {
	A string `json:"a"`
	B string `json:"b"`
}

type outer struct {
	inner
	C string `json:"c"`
}

type tagged struct {
	Inner inner `json:"inner"`
}

// An embedded untagged struct is inlined, as in json; a tagged one nests.
func TestRenderGenericInlinesEmbedded(t *testing.T) {
	var sb strings.Builder
	if err := renderGeneric(&sb, NewStyles(false), outer{inner{"1", "2"}, "3"}); err != nil {
		t.Fatal(err)
	}
	if want := "a: 1\nb: 2\nc: 3\n"; sb.String() != want {
		t.Errorf("got:\n%s\nwant:\n%s", sb.String(), want)
	}
	sb.Reset()
	if err := renderGeneric(&sb, NewStyles(false), []outer{{inner{"1", "2"}, "3"}}); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(sb.String(), "A  B  C\n1  2  3") {
		t.Errorf("table:\n%s", sb.String())
	}
	sb.Reset()
	if err := renderGeneric(&sb, NewStyles(false), tagged{inner{"1", "2"}}); err != nil {
		t.Fatal(err)
	}
	if want := "inner:\n  a: 1\n  b: 2\n"; sb.String() != want {
		t.Errorf("tagged:\n%s", sb.String())
	}
}

type row struct {
	Name  string  `json:"name"`
	Items []inner `json:"items"`
}

// A nested section of a row with an empty first column has no leading space.
func TestRenderGenericNestedLabel(t *testing.T) {
	var sb strings.Builder
	if err := renderGeneric(&sb, NewStyles(false), []row{{"", []inner{{"x", "y"}}}, {"r2", []inner{{"z", "w"}}}}); err != nil {
		t.Fatal(err)
	}
	out := sb.String()
	if strings.Contains(out, "\n items:") || !strings.Contains(out, "\nitems:\n") || !strings.Contains(out, "\nr2 items:\n") {
		t.Errorf("got:\n%q", out)
	}
}
