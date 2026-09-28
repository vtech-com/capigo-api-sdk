package cmd

import "testing"

// The reorder flags the page promises have to exist, or the help lies. The
// body-coverage guard cannot see them: `boards lists update` registers
// --from-json, and that escape hatch short-circuits the per-field assertion for
// the whole command.
func TestBoardListsUpdateReorderFlags(t *testing.T) {
	for _, name := range []string{"after-list-id", "before-list-id"} {
		if boardListsUpdateCmd.Flags().Lookup(name) == nil {
			t.Errorf("boards lists update has no --%s flag", name)
		}
	}
}

// The delete command is declared in a different file from the group it belongs
// to, so a missing AddCommand would leave it unreachable with nothing failing:
// the help-skeleton test only walks commands that are in the tree.
func TestBoardListsDeleteIsRegistered(t *testing.T) {
	if boardListsDeleteCmd.Flags().Lookup("tenant") == nil {
		t.Error("boards lists delete has no --tenant flag")
	}

	for _, child := range boardListsCmd.Commands() {
		if child == boardListsDeleteCmd {
			return
		}
	}
	t.Error("boards lists delete is not a child of boards lists")
}
