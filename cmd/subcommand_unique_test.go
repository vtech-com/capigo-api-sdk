package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

// TestNoSubcommandIsRegisteredTwice walks the whole command tree. A command added
// to its parent twice shows up twice in help and makes the second registration
// dead code; it happens when two branches each extend the same AddCommand call.
func TestNoSubcommandIsRegisteredTwice(t *testing.T) {
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		seen := map[string]bool{}
		for _, sub := range c.Commands() {
			if seen[sub.Name()] {
				t.Errorf("%q registers the subcommand %q more than once", c.CommandPath(), sub.Name())
			}
			seen[sub.Name()] = true
			walk(sub)
		}
	}
	walk(rootCmd)
}
