package main

import (
	"regexp"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/nikeee/store-cli/internal/cli"
)

var resultLine = regexp.MustCompile(`^Result: \[?([A-Za-z]+)\]?$`)

// TestEveryCommandNamesItsResult checks that the Long text of every leaf
// command ends with one line "Result: <Type>" or "Result: [<Type>]" and that
// the type is documented in results.md.
func TestEveryCommandNamesItsResult(t *testing.T) {
	names := resultNames()
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		switch c.Name() {
		case "help", "completion":
			return
		}
		if len(c.Commands()) > 0 {
			for _, sub := range c.Commands() {
				walk(sub)
			}
			return
		}
		lines := strings.Split(c.Long, "\n")
		m := resultLine.FindStringSubmatch(lines[len(lines)-1])
		if m == nil {
			t.Errorf("%s: Long does not end with a line \"Result: <Type>\"", c.CommandPath())
			return
		}
		if !names[m[1]] {
			t.Errorf("%s: result type %q is not in resultTypes", c.CommandPath(), m[1])
		}
	}
	walk(cli.NewRootCmd(cli.DefaultDeps()))
}

func TestResultTypesAreUnique(t *testing.T) {
	seen := map[string]bool{}
	for _, r := range resultTypes {
		if seen[r.Name] {
			t.Errorf("duplicate result type %q", r.Name)
		}
		seen[r.Name] = true
	}
}
