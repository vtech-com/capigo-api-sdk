package cmd

import (
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

// requiredSlots are the sections every command page carries, in this order.
//
// The order matters as much as the presence: it is the order a caller needs
// them in. What is this for, how do I spell the call, what does each flag do,
// what comes back.
var requiredSlots = []string{"PURPOSE", "USAGE", "FLAGS", "OUTPUT"}

// retiredSlots are headings the pages no longer carry. A caveat belongs to the
// flag it qualifies; an example belongs beside the flag it demonstrates; the
// command tree already says what the sibling commands are; and INPUT was the
// flag list under another name.
var retiredSlots = []string{"INPUT", "CAVEATS", "EXAMPLES", "SEE ALSO"}

// cobraAuthoredCommands are the pages cobra writes, not us.
var cobraAuthoredCommands = map[string]bool{"help": true, "completion": true}

// callableCommands are the pages that document a call: everything a caller can
// actually run. A pure group documents a noun and a help topic documents a
// contract; neither gets the skeleton. `tasks comments` is both — it runs, and
// it has children — so it does.
//
// The set is captured once, at package init, because enableUnknownSubcommandErrors
// gives every group a RunE. Execute() calls it in production, and
// unknown_command_test.go calls it here; either way, asking Runnable() after
// that has happened would sweep every group into the skeleton check, and would
// do so only when the tests ran in a particular order.
var callableCommands = func() []*cobra.Command {
	rootCmd.InitDefaultHelpCmd()
	rootCmd.InitDefaultCompletionCmd()

	var out []*cobra.Command
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if cobraAuthoredCommands[c.Name()] && c.Parent() == rootCmd {
			return
		}
		if c != rootCmd && c.Runnable() {
			out = append(out, c)
		}
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(rootCmd)
	return out
}()

func leafCommands(t *testing.T) []*cobra.Command {
	t.Helper()
	return callableCommands
}

func TestLeafCommandsFollowTheSkeleton(t *testing.T) {
	for _, c := range leafCommands(t) {
		checkSkeleton(t, c)
	}
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

// The retired headings are easy to reintroduce by habit, and each one puts a
// fact somewhere a reader has to go looking for it.
func TestPagesDoNotCarryRetiredSections(t *testing.T) {
	for _, c := range leafCommands(t) {
		for _, line := range strings.Split(c.Long, "\n") {
			for _, slot := range retiredSlots {
				if line == slot {
					t.Errorf("%q carries the retired %s section", c.CommandPath(), slot)
				}
			}
		}
	}
}

// Cobra no longer appends a Flags block (see help_render.go), so a flag that
// the FLAGS section forgets is a flag with no documentation anywhere.
func TestFlagsSectionDocumentsEveryFlag(t *testing.T) {
	// Global flags are documented once, on the root page. A page restates -o
	// when it changes what that command returns, and omits it where the command
	// ignores it (version, config) — so it cannot be required here.
	global := map[string]bool{"api-url": true, "verbose": true, "help": true, "output": true}

	for _, c := range leafCommands(t) {
		section := sectionOf(c.Long, "FLAGS")
		if section == "" {
			continue // reported by checkSkeleton
		}
		c.Flags().VisitAll(func(f *pflag.Flag) {
			if global[f.Name] {
				return
			}
			if !strings.Contains(section, "--"+f.Name) {
				t.Errorf("%q defines --%s but FLAGS does not document it", c.CommandPath(), f.Name)
			}
		})
	}
}

// The reverse worry: a page that names a flag the command does not define
// sends the reader to run something that will be rejected.
func TestHelpDoesNotNameFlagsThatDoNotExist(t *testing.T) {
	global := map[string]bool{
		"output": true, "api-url": true, "verbose": true, "help": true, "tenant": true,
	}

	for _, c := range leafCommands(t) {
		// Only FLAGS is checked: OUTPUT and PURPOSE routinely mention another
		// command's flags on purpose.
		input := sectionOf(c.Long, "FLAGS")
		if input == "" {
			continue
		}
		for _, name := range flagNamesIn(input) {
			if global[name] || c.Flags().Lookup(name) != nil {
				continue
			}
			t.Errorf("%q documents --%s in FLAGS, but defines no such flag", c.CommandPath(), name)
		}
	}
}

// Every page a caller reads should show the call it is reading about. A USAGE
// line that does not begin with the command's own path is describing something
// else.
func TestUsageSectionShowsThisCommand(t *testing.T) {
	for _, c := range leafCommands(t) {
		usage := sectionOf(c.Long, "USAGE")
		if usage == "" {
			continue
		}
		want := "capigo " + c.CommandPath()[len("capigo "):]
		if !strings.Contains(usage, want) {
			t.Errorf("%q has a USAGE section that never spells %q", c.CommandPath(), want)
		}
	}
}

// Command sorting is off, so a group's children display in the order they were
// registered — and init() runs in file-name order. A group whose children are
// spread across two files therefore lists them in an order nobody chose:
// `tasks attachments` sorted above `tasks list` until it was registered from
// tasks.go on purpose. Reading comes before writing, so `list` leads.
func TestListIsTheFirstCommandInItsGroup(t *testing.T) {
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		var visible []*cobra.Command
		for _, sub := range c.Commands() {
			if sub.IsAvailableCommand() {
				visible = append(visible, sub)
			}
		}
		hasList := false
		for _, sub := range visible {
			if sub.Name() == "list" {
				hasList = true
			}
		}
		if hasList && visible[0].Name() != "list" {
			t.Errorf("%q lists %q before %q; registration order is display order",
				c.CommandPath(), visible[0].Name(), "list")
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
		for _, other := range requiredSlots {
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
