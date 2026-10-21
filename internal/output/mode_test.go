package output_test

import (
	"testing"

	"github.com/nikeee/store-cli/internal/clierr"
	"github.com/nikeee/store-cli/internal/output"
)

func mapEnv(m map[string]string) func(string) (string, bool) {
	return func(k string) (string, bool) { v, ok := m[k]; return v, ok }
}

func TestResolveModePrecedence(t *testing.T) {
	tests := []struct {
		name string
		flag string
		env  map[string]string
		want output.Mode
	}{
		{"flag wins", "json", map[string]string{"STORE_OUTPUT": "toon", "CLAUDECODE": "1"}, output.ModeJSON},
		{"env beats marker", "", map[string]string{"STORE_OUTPUT": "text", "AI_AGENT": "x"}, output.ModeText},
		{"empty marker counts", "", map[string]string{"AI_AGENT": ""}, output.ModeTOON},
		{"default", "", nil, output.ModeText},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := output.ResolveMode(tt.flag, mapEnv(tt.env))
			if err != nil || got != tt.want {
				t.Fatalf("got %q, %v; want %q", got, err, tt.want)
			}
		})
	}
}

func TestResolveModeInvalid(t *testing.T) {
	for _, tt := range []struct {
		flag string
		env  map[string]string
	}{{"xml", nil}, {"", map[string]string{"STORE_OUTPUT": "xml"}}} {
		_, err := output.ResolveMode(tt.flag, mapEnv(tt.env))
		if err == nil || clierr.ExitCode(err) != 2 {
			t.Fatalf("err=%v", err)
		}
	}
}
