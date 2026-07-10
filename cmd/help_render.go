// Package cmd — help_render.go
//
// One function renders every help page. Cobra's own template appends a Usage
// and a Flags block to whatever a command's Long text already says; on a page
// whose Long text documents its flags, that block is a second, differently
// worded copy of the same truth, and the two drift. So the help function is
// replaced rather than the template extended.
//
// What a page contains now depends on what the command is:
//
//	leaf command   its Long text, and nothing else. PURPOSE, USAGE, FLAGS and
//	               OUTPUT are authored there, in that order.
//	group / root   its Long text, then the commands beneath it, generated from
//	               the tree so the list cannot go stale.
//	root only      the global flags, generated from their definitions, and the
//	               help topics.
//	help topic     its Long text; a topic has neither flags nor children.
//
// Every page ends with the build footer.
package cmd

import (
	"fmt"
	"io"
	"strings"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/version"
)

// renderHelp writes one help page. It is installed on the root command, and
// cobra resolves a command's help function by walking up to its parent, so
// this runs for every command, group, and help topic in the tree.
func renderHelp(c *cobra.Command, w io.Writer) {
	body := c.Long
	if strings.TrimSpace(body) == "" {
		body = c.Short
	}
	_, _ = fmt.Fprintln(w, strings.TrimRight(body, "\n \t"))

	// A group lists what is under it. `enableUnknownSubcommandErrors` gives
	// groups a RunE, so Runnable() is true for them by the time a page is
	// printed — the presence of children is the only reliable test.
	if c.HasSubCommands() {
		if rows := commandRows(c); rows != "" {
			_, _ = fmt.Fprintf(w, "\nCOMMANDS\n%s", rows)
		}
	}

	if c == rootCmd {
		// The help flag is added lazily, at execute time; asking for it here
		// means the page never omits the flag the reader just used.
		c.InitDefaultHelpFlag()
		_, _ = fmt.Fprintf(w, "\nFLAGS\n%s", strings.TrimRight(c.LocalFlags().FlagUsages(), "\n")+"\n")
		if rows := helpTopicRows(c); rows != "" {
			_, _ = fmt.Fprintf(w, "\nHELP TOPICS\n%s", rows)
		}
	}

	_, _ = fmt.Fprint(w, helpFooter(version.Version, version.Date))
}

// commandRows renders the visible children of a group, in the order they were
// registered. Command sorting is disabled (see init), so that order is the
// order a caller meets them in: read before write, create before destroy.
func commandRows(c *cobra.Command) string {
	width := 0
	for _, sub := range c.Commands() {
		if sub.IsAvailableCommand() && len(sub.Name()) > width {
			width = len(sub.Name())
		}
	}
	var b strings.Builder
	for _, sub := range c.Commands() {
		if !sub.IsAvailableCommand() {
			continue
		}
		fmt.Fprintf(&b, "  %-*s  %s\n", width, sub.Name(), sub.Short)
	}
	return b.String()
}

// helpTopicRows lists the pages that document a fact true of many commands.
// They are reachable only from here, so the root is the only page that may
// omit them.
func helpTopicRows(c *cobra.Command) string {
	width := 0
	for _, sub := range c.Commands() {
		if sub.IsAdditionalHelpTopicCommand() && len(sub.Name()) > width {
			width = len(sub.Name())
		}
	}
	var b strings.Builder
	for _, sub := range c.Commands() {
		if !sub.IsAdditionalHelpTopicCommand() {
			continue
		}
		fmt.Fprintf(&b, "  capigo help %-*s  %s\n", width, sub.Name(), sub.Short)
	}
	return b.String()
}

func init() {
	// Registration order is meaningful — see commandRows.
	cobra.EnableCommandSorting = false

	rootCmd.SetHelpFunc(func(c *cobra.Command, _ []string) {
		renderHelp(c, c.OutOrStdout())
	})

	// `completion` is cobra's, documents a shell integration rather than the
	// API, and would sit among the domains in COMMANDS. `capigo completion`
	// still works and still prints its own page.
	rootCmd.CompletionOptions.HiddenDefaultCmd = true

	// Same for `help`: `capigo help output` must keep working, but the command
	// itself is a CLI convention, not a domain. Cobra only installs its default
	// help command when none is set, so setting one here replaces it.
	rootCmd.SetHelpCommand(&cobra.Command{
		Use:    "help [command]",
		Short:  "Help about any command",
		Hidden: true,
		Run: func(c *cobra.Command, args []string) {
			target, _, err := c.Root().Find(args)
			if target == nil || err != nil {
				c.Printf("Unknown help topic %#q\n", args)
				_ = c.Root().Usage()
				return
			}
			target.InitDefaultHelpFlag()
			_ = target.Help()
		},
	})
}
