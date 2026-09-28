package cmd

import "testing"

// `boards delete` is declared away from the group it belongs to, so a missing
// AddCommand would leave it unreachable with nothing failing: the help-skeleton
// test only walks commands that are in the tree.
func TestBoardsDeleteIsRegistered(t *testing.T) {
	if boardsDeleteCmd.Flags().Lookup("tenant") == nil {
		t.Error("boards delete has no --tenant flag")
	}

	for _, child := range boardCmd.Commands() {
		if child == boardsDeleteCmd {
			return
		}
	}
	t.Error("boards delete is not a child of boards")
}
