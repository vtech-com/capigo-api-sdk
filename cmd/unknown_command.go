package cmd

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/spf13/cobra"
)

// -----------------------------------------------------------------------------
// Layer 1 — make cobra's own "unknown command" detection fire for subcommand
// groups, not just the root.
//
// Cobra's default Args validator (legacyArgs, see spf13/cobra args.go) only
// raises "unknown command %q for %q" at the root: `!cmd.HasParent()`. For a
// group command such as "products" or "variants" — which has subcommands but
// no Run/RunE of its own — unmatched trailing args are accepted without error,
// cmd.Runnable() is false, and cobra's execute() takes the
// `if !c.Runnable() { return flag.ErrHelp }` branch: it silently prints the
// group's help with exit 0. Verified empirically (see PR2 spec) with
// `capigo variants update foo` before this fix: no error, no suggestion,
// exit 0. That swallows both cobra's own edit-distance suggestions and the
// curated Layer 2 redirect below for anything below the root.
//
// enableUnknownSubcommandErrors walks the full command tree once, after all
// commands have been registered, and gives every non-runnable group command a
// RunE that mirrors cobra's own root-level message (including cobra's real
// SuggestionsFor distance computation — reused, not reimplemented) for any
// unmatched subcommand-like argument. A bare invocation of the group (no args)
// still just prints help, unchanged from today.
// -----------------------------------------------------------------------------

func enableUnknownSubcommandErrors(c *cobra.Command) {
	for _, child := range c.Commands() {
		enableUnknownSubcommandErrors(child)
	}
	if !c.HasSubCommands() || c.Runnable() {
		return
	}
	c.RunE = func(cmd *cobra.Command, args []string) error {
		if len(args) == 0 {
			return cmd.Help()
		}
		return fmt.Errorf("unknown command %q for %q%s", args[0], cmd.CommandPath(), suggestionsBlock(cmd, args[0]))
	}
}

// suggestionsBlock reproduces cobra's unexported findSuggestions formatting
// (see spf13/cobra command.go) on top of the exported SuggestionsFor, so the
// "Did you mean this?" text stays byte-for-byte consistent with cobra's own
// root-level output.
func suggestionsBlock(c *cobra.Command, arg string) string {
	if c.DisableSuggestions {
		return ""
	}
	// Self-heal the distance threshold exactly as cobra's own (unexported)
	// findSuggestions does. The *exported* SuggestionsFor does NOT do this, so
	// with the zero-value default only prefix matches would suggest — a real
	// typo like "gett" (distance 1 from "get") would return nothing. Mirror
	// cobra so group-level typos get the same edit-distance suggestions as the
	// root does.
	if c.SuggestionsMinimumDistance <= 0 {
		c.SuggestionsMinimumDistance = 2
	}
	var sb strings.Builder
	if suggestions := c.SuggestionsFor(arg); len(suggestions) > 0 {
		sb.WriteString("\n\nDid you mean this?\n")
		for _, s := range suggestions {
			_, _ = fmt.Fprintf(&sb, "\t%v\n", s)
		}
	}
	return sb.String()
}

// -----------------------------------------------------------------------------
// Layer 2 — curated cross-group redirects.
//
// Cobra's edit-distance suggestions (Layer 1) only catch typos of a sibling
// command name. They cannot catch a conceptual wrong-guess where the right
// command lives in a *different* group entirely — e.g. someone reaches for
// "variants update" when variants are read-only and the write path is
// "products variants". Add a row here only when (a) it's a conceptual miss
// Layer 1 can't express and (b) there is evidence it actually happens. Keep
// this table short.
// -----------------------------------------------------------------------------

// unknownCommandRedirect is one curated entry: a family of plausible-but-wrong
// subcommands under `group`, and the Next hint pointing at the real command.
// targetPath is the space-separated path of the real command this hint points
// at (e.g. "products variants"); it exists purely so the anti-rot test can
// assert the target is still a real command in the tree.
type unknownCommandRedirect struct {
	group      string
	subs       []string
	next       string
	targetPath string
}

var unknownCommandRegistry = []unknownCommandRedirect{
	{
		group: "variants",
		subs:  []string{"update", "create", "replace"},
		next: "The 'variants' group is read-only. To create or update a variant, upsert it through " +
			"'capigo products variants --product-id <id> --from-json -' (a JSON array; an item with " +
			"variant_id updates, without creates).",
		targetPath: "products variants",
	},
	{
		group: "products",
		subs:  []string{"delete", "remove", "destroy"},
		next: "Products have no delete command. To take a product out of circulation, archive it: " +
			"'capigo products update <id> --from-json -' with {\"status\":\"ARCHIVED\"}.",
		targetPath: "products update",
	},
}

// removedFlagHints answers a caller who is still spelling a flag this CLI used
// to have. cobra's own message — `unknown shorthand flag: 'o' in -o` — names
// the character it choked on and nothing else, which reads like a typo rather
// than like a flag that was deliberately removed. A caller who thinks it is a
// typo tries again.
var removedFlagHints = []struct{ marker, next string }{
	{
		marker: "shorthand flag: 'o'",
		next: "There is no --output flag: capigo prints JSON on stdout, always. " +
			"Drop the flag. The result is at .data and the tenant is at .meta.tenant. " +
			"See 'capigo help output'.",
	},
	{
		marker: "unknown flag: --output",
		next: "There is no --output flag: capigo prints JSON on stdout, always. " +
			"Drop the flag. The result is at .data and the tenant is at .meta.tenant. " +
			"See 'capigo help output'.",
	},
}

// nextForRemovedFlag returns the hint for a flag this CLI no longer defines.
// Pure string→string, like redirectForUnknownCommand below.
func nextForRemovedFlag(errMsg string) (next string, matched bool) {
	for _, h := range removedFlagHints {
		if strings.Contains(errMsg, h.marker) {
			return h.next, true
		}
	}
	return "", false
}

// unknownCommandPattern parses cobra's "unknown command %q for %q" message
// (see spf13/cobra args.go legacyArgs and cmd/unknown_command.go
// enableUnknownSubcommandErrors above, which produces the same shape for
// subcommand groups). Group 1 is the attempted subcommand; group 2 is the
// command path below "capigo" (empty for a root-level miss, e.g.
// `unknown command "foo" for "capigo"`).
var unknownCommandPattern = regexp.MustCompile(`^unknown command "([^"]+)" for "capigo(?: (.+))?"`)

// redirectForUnknownCommand parses a cobra-shaped "unknown command" error
// message and, if it matches a curated Layer 2 entry, returns the Next hint.
// Pure string→string: no cobra, no I/O, safe to unit test directly.
func redirectForUnknownCommand(errMsg string) (next string, matched bool) {
	m := unknownCommandPattern.FindStringSubmatch(errMsg)
	if m == nil {
		return "", false
	}
	sub, group := m[1], m[2]
	for _, r := range unknownCommandRegistry {
		if r.group != group {
			continue
		}
		for _, s := range r.subs {
			if strings.EqualFold(s, sub) {
				return r.next, true
			}
		}
	}
	return "", false
}
