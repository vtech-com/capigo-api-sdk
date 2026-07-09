package cmd

import (
	"strings"
	"testing"
)

// helpTopicNames are the cross-cutting pages a command help page may reference.
// A command page that says `capigo help output` and finds nothing is a page that
// lies, so the set is asserted rather than assumed.
var helpTopicNames = []string{
	"tenancy",
	"output",
	"exit-codes",
	"soft-delete",
	"versioning",
}

func TestHelpTopicsAreRegisteredAndNotRunnable(t *testing.T) {
	for _, name := range helpTopicNames {
		var found bool
		for _, c := range rootCmd.Commands() {
			if c.Name() != name {
				continue
			}
			found = true

			// A help topic must never execute. Cobra decides this by the absence
			// of Run/RunE, and only then lists it under "Additional help topics".
			if c.Runnable() {
				t.Errorf("help topic %q is runnable; it must have no Run/RunE", name)
			}
			if !c.IsAdditionalHelpTopicCommand() {
				t.Errorf("help topic %q is not treated as an additional help topic by cobra", name)
			}
			if strings.TrimSpace(c.Long) == "" {
				t.Errorf("help topic %q has an empty Long; the page is the whole point", name)
			}
			if strings.TrimSpace(c.Short) == "" {
				t.Errorf("help topic %q has an empty Short; it labels the root listing", name)
			}
		}
		if !found {
			t.Errorf("help topic %q is not registered on rootCmd", name)
		}
	}
}

// Every `capigo help …` invocation runs the built-in, runnable `help` command,
// so it passes through PersistentPreRunE and would otherwise emit the -o json
// nudge on stderr. Documentation is not a resource; the nudge there is a false
// positive, and false positives train the reader to ignore the hint where it
// matters.
func TestHelpCommandIsExemptFromJSONHint(t *testing.T) {
	// Cobra only attaches its built-in `help` command inside Execute(), so a test
	// that reaches for it beforehand must ask for it explicitly.
	rootCmd.InitDefaultHelpCmd()

	helpCmd, _, err := rootCmd.Find([]string{"help"})
	if err != nil {
		t.Fatalf("rootCmd.Find([help]): %v", err)
	}
	if helpCmd.Name() != "help" {
		t.Fatalf("expected the help command, got %q", helpCmd.Name())
	}
	if !cmdExemptFromJSONHint(helpCmd) {
		t.Error("`capigo help …` must be exempt from the -o json hint")
	}
}

// The help footer names the build a page came from. A local `go build` injects
// no date, and a bare "(built unknown)" would be noise rather than information.
func TestHelpFooterOmitsUnknownBuildDate(t *testing.T) {
	tests := []struct {
		name        string
		version     string
		date        string
		wantBuilt   bool
		wantVersion string
	}{
		{name: "release build", version: "0.20.1", date: "2026-06-18", wantBuilt: true, wantVersion: "0.20.1"},
		{name: "local build", version: "dev", date: "unknown", wantBuilt: false, wantVersion: "dev"},
		{name: "empty date", version: "dev", date: "", wantBuilt: false, wantVersion: "dev"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := helpFooter(tc.version, tc.date)
			if !strings.Contains(got, tc.wantVersion) {
				t.Errorf("footer %q does not name version %q", got, tc.wantVersion)
			}
			if strings.Contains(got, "built") != tc.wantBuilt {
				t.Errorf("footer %q: wantBuilt=%v", got, tc.wantBuilt)
			}
			if !strings.Contains(got, "capigo help versioning") {
				t.Errorf("footer %q does not point at the versioning topic", got)
			}
		})
	}
}

// A topic page that references a sibling topic which does not exist is the same
// class of defect as a command page that lies about its output: the reader
// follows the pointer and finds nothing.
func TestHelpTopicCrossReferencesResolve(t *testing.T) {
	known := map[string]bool{}
	for _, n := range helpTopicNames {
		known[n] = true
	}
	for _, c := range rootCmd.Commands() {
		if !known[c.Name()] {
			continue
		}
		for _, line := range strings.Split(c.Long, "\n") {
			idx := strings.Index(line, "capigo help ")
			if idx < 0 {
				continue
			}
			rest := strings.TrimSpace(line[idx+len("capigo help "):])
			target := strings.Fields(rest)[0]
			if !known[target] {
				t.Errorf("topic %q references `capigo help %s`, which is not a registered topic", c.Name(), target)
			}
		}
	}
}
