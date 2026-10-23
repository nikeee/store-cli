package clierr_test

import (
	"errors"
	"fmt"
	"slices"
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

func TestCollector(t *testing.T) {
	var c clierr.Collector
	if c.Err() != nil {
		t.Fatal("empty collector is an error")
	}
	c.Add(nil)
	c.Add(clierr.At("de-DE/name.txt", clierr.Issue{File: "meta/de-DE/name.txt", Line: 1, Col: 31, Message: "too long"}))
	c.Add(clierr.At("en-US/name.txt", clierr.Issue{Message: "too long"}))
	c.Add(fmt.Errorf("de-DE: %w", clierr.New("phone needs 2 screenshots")))
	c.Add(fmt.Errorf("de-DE: %w", clierr.Filef("a.png", "has an alpha channel")))
	err := c.Err()
	want := "de-DE/name.txt: too long; en-US/name.txt: too long; de-DE: phone needs 2 screenshots; de-DE: a.png: has an alpha channel"
	if err == nil || err.Error() != want || clierr.ExitCode(err) != 1 {
		t.Fatalf("err = %v", err)
	}
	var e *clierr.Error
	if !errors.As(err, &e) {
		t.Fatalf("not a *clierr.Error: %T", err)
	}
	wantIssues := []clierr.Issue{
		{File: "meta/de-DE/name.txt", Line: 1, Col: 31, Message: "too long"},
		{Message: "en-US/name.txt: too long"},
		{Message: "de-DE: phone needs 2 screenshots"},
		{File: "a.png", Message: "has an alpha channel"},
	}
	if !slices.Equal(e.Issues, wantIssues) {
		t.Errorf("issues = %+v", e.Issues)
	}
}
