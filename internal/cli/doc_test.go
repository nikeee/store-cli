package cli

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestEveryCommandHasExample keeps the generated reference complete: every
// command that does work (not a group, not cobra's help and completion) has a
// Short, Long and Example text.
func TestEveryCommandHasExample(t *testing.T) {
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
		path := c.CommandPath()
		if c.Short == "" {
			t.Errorf("%s: empty Short", path)
		}
		if c.Long == "" {
			t.Errorf("%s: empty Long", path)
		}
		if c.Example == "" {
			t.Errorf("%s: empty Example", path)
		}
	}
	walk(NewRootCmd(DefaultDeps()))
}
