package cmd

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
	"golang.org/x/term"
)

// jsonHintText is the advisory printed when output is being captured but the
// default table mode is still in effect. It names the exact failure (table is
// not JSON) and the fix, and points at the silence switch.
const jsonHintText = "[capigo] stdout is not a terminal and --output was not set, so this is table text, not JSON. " +
	"If you intend to parse it, re-run with -o json. (set CAPIGO_NO_HINTS=1 to silence)"

// shouldHintJSON decides whether to nudge toward -o json. It is the pure core of
// maybeWarnNonTTYTable, split out so the decision can be tested without a real
// terminal. The nudge fires only when output is being captured (stdout is not a
// TTY) while the default table mode is still in effect and the user has not
// silenced hints — i.e. the one case where a parser is about to choke on text.
func shouldHintJSON(mode string, outputChanged, stdoutIsTTY, hintsDisabled bool) bool {
	if hintsDisabled || stdoutIsTTY || outputChanged {
		return false
	}
	return mode == "table"
}

// hintExemptGroups are top-level command groups whose output is not a
// JSON-parseable resource — they print a plain scalar or fixed text and ignore
// -o json — so nudging toward -o json there would be a false positive. A hint
// that is wrong on `version`/`config` trains the agent to ignore it on the
// commands where it matters, so exempt them. (`health`, `auth whoami/login`
// honour json, so they are NOT exempt.)
//
// `help` is exempt for the same reason: `capigo help <topic>` and
// `capigo help <command>` print documentation, never a resource. Note that the
// `--help` FLAG never reaches here — cobra short-circuits on it before
// PersistentPreRunE — but the `help` COMMAND is runnable and does, so without
// this entry every `capigo help …` invocation emits a spurious -o json nudge.
var hintExemptGroups = map[string]bool{"version": true, "config": true, "help": true, "logout": true}

// cmdExemptFromJSONHint reports whether cmd, or any of its ancestors, belongs to
// an exempt group — covering both `capigo version` and `capigo config get`.
func cmdExemptFromJSONHint(cmd *cobra.Command) bool {
	for c := cmd; c != nil; c = c.Parent() {
		if hintExemptGroups[c.Name()] {
			return true
		}
	}
	return false
}

// maybeWarnNonTTYTable emits the -o json nudge to STDERR when appropriate. It
// must never write to stdout: the whole point is to protect the captured stdout
// stream, so polluting it would defeat the purpose (and corrupt legitimate
// table pipes). This is the only guard for the redirect-table-as-JSON mistake,
// since that path exits 0 and the on-error diagnosis block never fires.
func maybeWarnNonTTYTable(cmd *cobra.Command) {
	if cmdExemptFromJSONHint(cmd) {
		return
	}
	hintsDisabled := os.Getenv("CAPIGO_NO_HINTS") != ""
	outputChanged := cmd.Flags().Changed("output")
	stdoutIsTTY := term.IsTerminal(int(os.Stdout.Fd()))
	if shouldHintJSON(outputMode, outputChanged, stdoutIsTTY, hintsDisabled) {
		_, _ = fmt.Fprintln(os.Stderr, jsonHintText)
	}
}
