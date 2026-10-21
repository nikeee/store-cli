package cli

import (
	"testing"

	"github.com/nikeee/store-cli/internal/clierr"
)

func TestParseBoolArg(t *testing.T) {
	for _, tc := range []struct {
		in      string
		want    bool
		wantErr bool
	}{
		{"true", true, false}, {"false", false, false}, {"True", false, true}, {"1", false, true}, {"", false, true},
	} {
		got, err := parseBoolArg("--x", tc.in)
		if got != tc.want || (err != nil) != tc.wantErr || (err != nil && clierr.ExitCode(err) != 2) {
			t.Errorf("parseBoolArg(%q) = %v, %v", tc.in, got, err)
		}
	}
}
