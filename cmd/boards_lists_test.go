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
