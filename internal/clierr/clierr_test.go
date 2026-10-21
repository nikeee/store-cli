package clierr_test

import (
	"errors"
	"fmt"
	"testing"

	"github.com/nikeee/store-cli/internal/clierr"
)

func TestExitCode(t *testing.T) {
	usage := clierr.Usagef("bad %s", "flag")
	tests := []struct {
		name string
		err  error
		want int
	}{
		{"nil", nil, 0},
		{"error", clierr.New("boom"), 1},
		{"usage", usage, 2},
		{"wrapped usage", fmt.Errorf("x: %w", usage), 2},
		{"plain", errors.New("plain"), 1},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := clierr.ExitCode(tt.err); got != tt.want {
				t.Fatalf("ExitCode = %d, want %d", got, tt.want)
			}
		})
	}
}
