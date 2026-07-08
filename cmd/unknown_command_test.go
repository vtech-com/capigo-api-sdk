package cmd

import (
	"encoding/json"
	"io"
	"os"
	"strings"
	"testing"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/api"
)

func TestRedirectForUnknownCommand(t *testing.T) {
	cases := []struct {
		name    string
		errMsg  string
		matched bool
		want    string // substring expected in the Next hint when matched
	}{
		{
			name:    "variants update",
			errMsg:  `unknown command "update" for "capigo variants"`,
			matched: true,
			want:    "products variants --product-id",
		},
		{
			name:    "variants create",
			errMsg:  `unknown command "create" for "capigo variants"`,
			matched: true,
			want:    "products variants --product-id",
		},
		{
			name:    "variants replace",
			errMsg:  `unknown command "replace" for "capigo variants"`,
			matched: true,
			want:    "products variants --product-id",
		},
		{
			name:    "variants replace case-insensitive",
			errMsg:  `unknown command "REPLACE" for "capigo variants"`,
			matched: true,
			want:    "products variants --product-id",
		},
		{
			name:    "products delete",
			errMsg:  `unknown command "delete" for "capigo products"`,
			matched: true,
			want:    "products update <id> --from-json",
		},
		{
			name:    "products remove",
			errMsg:  `unknown command "remove" for "capigo products"`,
			matched: true,
			want:    "products update <id> --from-json",
		},
		{
			name:    "products destroy",
			errMsg:  `unknown command "destroy" for "capigo products"`,
			matched: true,
			want:    "products update <id> --from-json",
		},
		{
			name:    "products destroy with cobra suggestion block appended",
			errMsg:  "unknown command \"destroy\" for \"capigo products\"\n\nDid you mean this?\n\tupdate\n",
			matched: true,
			want:    "products update <id> --from-json",
		},
		{
			name:    "unknown sub on an unrelated group does not misfire",
			errMsg:  `unknown command "delete" for "capigo boards"`,
			matched: false,
		},
		{
			name:    "unknown sub on variants not in the curated family",
			errMsg:  `unknown command "archive" for "capigo variants"`,
			matched: false,
		},
		{
			name:    "root-level unknown command",
			errMsg:  `unknown command "taskss" for "capigo"`,
			matched: false,
		},
		{
			name:    "normal server API error never misfires",
			errMsg:  "E9426: variant create failed (request_id=abc123)",
			matched: false,
		},
		{
			name:    "unrelated cobra error never misfires",
			errMsg:  "unknown flag: --tenant",
			matched: false,
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			next, matched := redirectForUnknownCommand(tc.errMsg)
			if matched != tc.matched {
				t.Fatalf("matched = %v, want %v (next=%q)", matched, tc.matched, next)
			}
			if matched && !strings.Contains(next, tc.want) {
				t.Fatalf("next = %q, want substring %q", next, tc.want)
			}
			if !matched && next != "" {
				t.Fatalf("expected empty next on no match, got %q", next)
			}
		})
	}
}

// findCommandPath walks the cobra command tree looking for the command
// addressed by the space-separated path (e.g. "products variants"), relative
// to root.
func findCommandPath(root *cobra.Command, path string) *cobra.Command {
	parts := strings.Fields(path)
	cur := root
	for _, part := range parts {
		var next *cobra.Command
		for _, child := range cur.Commands() {
			if child.Name() == part {
				next = child
				break
			}
		}
		if next == nil {
			return nil
		}
		cur = next
	}
	return cur
}

// TestUnknownCommandRegistryTargetsExist is the anti-rot guard: every curated
// Layer 2 redirect must point at a command that actually exists in the live
// cobra command tree. If a target command is ever renamed or removed, this
// test fails instead of the CLI silently pointing an agent (or a human) at a
// dead command.
func TestUnknownCommandRegistryTargetsExist(t *testing.T) {
	if len(unknownCommandRegistry) == 0 {
		t.Fatal("unknownCommandRegistry is empty — nothing to protect")
	}
	for _, entry := range unknownCommandRegistry {
		t.Run(entry.group+"->"+entry.targetPath, func(t *testing.T) {
			target := findCommandPath(rootCmd, entry.targetPath)
			if target == nil {
				t.Fatalf("redirect target %q for group %q does not exist in the command tree — "+
					"the hint would point at a dead command", entry.targetPath, entry.group)
			}
			// Also make sure the group this redirect fires for still exists —
			// a rename there would silently stop the hint from ever matching.
			if findCommandPath(rootCmd, entry.group) == nil {
				t.Fatalf("redirect source group %q no longer exists in the command tree", entry.group)
			}
		})
	}
}

// TestGroupLevelTypoSuggestsSibling drives the RunE that
// enableUnknownSubcommandErrors installs on a group command with a typo'd
// subcommand and asserts the resulting error carries cobra's edit-distance
// suggestion. Guards the SuggestionsMinimumDistance self-heal: the exported
// SuggestionsFor does NOT set the threshold, so without the self-heal a real
// typo like "gett" (distance 1 from "get") would return no suggestion.
func TestGroupLevelTypoSuggestsSibling(t *testing.T) {
	enableUnknownSubcommandErrors(rootCmd)

	products := findCommandPath(rootCmd, "products")
	if products == nil {
		t.Fatal("products group not found in command tree")
		return
	}
	if products.RunE == nil {
		t.Fatal("enableUnknownSubcommandErrors did not install a RunE on the products group")
		return
	}

	err := products.RunE(products, []string{"gett", "foo"})
	if err == nil {
		t.Fatal("expected an unknown-command error for a typo'd subcommand, got nil")
	}
	msg := err.Error()
	if !strings.Contains(msg, `unknown command "gett"`) {
		t.Errorf("error should name the unknown subcommand, got %q", msg)
	}
	if !strings.Contains(msg, "Did you mean this?") || !strings.Contains(msg, "get") {
		t.Errorf("error should suggest the sibling 'get' via edit distance, got %q", msg)
	}
}

// TestRenderCLIErrorEmitsNextForVariantsUpdate mirrors the existing
// error_catalog_test / error_rich_test patterns, but end to end: build the
// exact api.APIError shape Execute() constructs for a cobra error, call the
// real renderCLIError, and confirm error.next appears on stdout under
// -o json — the same field an agent would read.
func TestRenderCLIErrorEmitsNextForVariantsUpdate(t *testing.T) {
	prevMode := outputMode
	outputMode = "json"
	defer func() { outputMode = prevMode }()

	r, w, err := os.Pipe()
	if err != nil {
		t.Fatalf("os.Pipe: %v", err)
	}
	origStdout := os.Stdout
	os.Stdout = w
	defer func() { os.Stdout = origStdout }()

	wrapped := &api.APIError{
		Code:       "VALIDATION_ERROR",
		Message:    `unknown command "update" for "capigo variants"`,
		HTTPStatus: 400,
	}
	renderCLIError(wrapped)

	_ = w.Close()
	os.Stdout = origStdout
	out, err := io.ReadAll(r)
	if err != nil {
		t.Fatalf("read pipe: %v", err)
	}

	var got struct {
		Error struct {
			Next           string `json:"next"`
			CapabilityNote bool   `json:"capability_note"`
		} `json:"error"`
	}
	if err := json.Unmarshal(out, &got); err != nil {
		t.Fatalf("stdout is not valid JSON: %v\n%s", err, out)
	}
	if !strings.Contains(got.Error.Next, "products variants --product-id") {
		t.Errorf("error.next missing redirect hint, got %q", got.Error.Next)
	}
	if got.Error.CapabilityNote {
		t.Errorf("capability_note must stay false for a client-side/cobra redirect")
	}
}
