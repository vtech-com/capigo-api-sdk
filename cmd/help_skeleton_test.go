package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// requiredSlots are the sections every command page carries, in this order.
// CAVEATS is a legitimate sixth slot but is only present where a command has a
// trap worth naming, so it is not required.
//
// The order matters as much as the presence: a reader who has learned one page
// can then skim any other, and an agent can extract a section without guessing
// where it starts.
var requiredSlots = []string{"PURPOSE", "INPUT", "OUTPUT", "EXAMPLES", "SEE ALSO"}

// cobraAuthoredCommands are the pages cobra writes, not us.
var cobraAuthoredCommands = map[string]bool{"help": true, "completion": true}

// A command that runs is a command a caller must know how to call, so it needs
// the full skeleton. Groups and help topics do not run, and are documented as
// prose.
func TestRunnableCommandsFollowTheSkeleton(t *testing.T) {
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultCompletionCmd()

	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if cobraAuthoredCommands[c.Name()] && c.Parent() == rootCmd {
			return
		}
		if c.Runnable() && c != rootCmd {
			checkSkeleton(t, c)
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(rootCmd)
}

func checkSkeleton(t *testing.T, c *cobra.Command) {
	t.Helper()

	if strings.TrimSpace(c.Long) == "" {
		t.Errorf("%q has no long help; a caller cannot build a call from a Short line", c.CommandPath())
		return
	}

	// Slot headers sit alone on their own line. A trailing parenthetical would
	// make the page unparseable by anything but a human.
	seen := map[string]int{}
	for i, line := range strings.Split(c.Long, "\n") {
		for _, slot := range requiredSlots {
			if line == slot {
				if _, dup := seen[slot]; dup {
					t.Errorf("%q repeats the %s header", c.CommandPath(), slot)
				}
				seen[slot] = i
			}
		}
	}

	prev := -1
	for _, slot := range requiredSlots {
		at, ok := seen[slot]
		if !ok {
			t.Errorf("%q is missing the %s section", c.CommandPath(), slot)
			continue
		}
		if at < prev {
			t.Errorf("%q has %s out of order", c.CommandPath(), slot)
		}
		prev = at
	}
}

// A page that names a flag the command does not define sends the reader to run
// something that will be rejected. This checks the reverse of the usual worry:
// not that a flag is undocumented, but that the documentation invents one.
func TestHelpDoesNotNameFlagsThatDoNotExist(t *testing.T) {
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultCompletionCmd()

	// Flags every command inherits, plus the ones a page may name while talking
	// about a *different* command in SEE ALSO or a cross-reference.
	global := map[string]bool{
		"output": true, "api-url": true, "verbose": true, "help": true, "tenant": true,
	}

	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if cobraAuthoredCommands[c.Name()] && c.Parent() == rootCmd {
			return
		}
		if c.Runnable() && c != rootCmd {
			// Only the INPUT section is checked: SEE ALSO and CAVEATS routinely
			// mention another command's flags on purpose.
			if input := sectionOf(c.Long, "INPUT"); input != "" {
				for _, name := range flagNamesIn(input) {
					if global[name] || c.Flags().Lookup(name) != nil {
						continue
					}
					t.Errorf("%q documents --%s in INPUT, but defines no such flag", c.CommandPath(), name)
				}
			}
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(rootCmd)
}

// sectionOf returns the body of one slot, up to the next slot header.
func sectionOf(long, slot string) string {
	lines := strings.Split(long, "\n")
	start := -1
	for i, l := range lines {
		if l == slot {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return ""
	}
	for i := start; i < len(lines); i++ {
		for _, other := range append(requiredSlots, "CAVEATS") {
			if lines[i] == other {
				return strings.Join(lines[start:i], "\n")
			}
		}
	}
	return strings.Join(lines[start:], "\n")
}

// flagNamesIn pulls every --long-flag out of a chunk of help text.
func flagNamesIn(text string) []string {
	var out []string
	for _, field := range strings.Fields(text) {
		if !strings.HasPrefix(field, "--") || len(field) < 3 {
			continue
		}
		name := strings.TrimPrefix(field, "--")
		// Trim the punctuation a sentence leaves behind: `--all,` `--tags.` `--ids"`
		name = strings.TrimRight(name, ".,;:\"')")
		if name == "" || strings.ContainsAny(name, "<>|") {
			continue
		}
		out = append(out, name)
	}
	return out
}
