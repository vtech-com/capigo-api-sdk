package cmd

import "testing"

// The flags these commands document in their own help have to exist, or the
// help lies. The body-coverage guard checks the flags against the spec's body
// fields; this checks that every one of the four takes a tenant — a board is
// addressed inside one workspace, so a call without one cannot be placed.
func TestBoardMembersCommandsCarryTheirFlags(t *testing.T) {
	commands := map[string]struct {
		command string
		want    []string
	}{
		"boards members list":   {"capigo boards members list", []string{"tenant", "page", "limit"}},
		"boards members add":    {"capigo boards members add", []string{"tenant", "user-id", "role"}},
		"boards members update": {"capigo boards members update", []string{"tenant", "role"}},
		"boards members remove": {"capigo boards members remove", []string{"tenant"}},
	}

	byName := map[string]func(string) bool{
		"capigo boards members list":   func(n string) bool { return boardMembersListCmd.Flags().Lookup(n) != nil },
		"capigo boards members add":    func(n string) bool { return boardMembersAddCmd.Flags().Lookup(n) != nil },
		"capigo boards members update": func(n string) bool { return boardMembersUpdateCmd.Flags().Lookup(n) != nil },
		"capigo boards members remove": func(n string) bool { return boardMembersRemoveCmd.Flags().Lookup(n) != nil },
	}

	for name, entry := range commands {
		hasFlag, ok := byName[entry.command]
		if !ok {
			t.Fatalf("%s: no command to check; the map and the commands have drifted", name)
		}
		for _, flag := range entry.want {
			if !hasFlag(flag) {
				t.Errorf("%s has no --%s flag", entry.command, flag)
			}
		}
	}
}

// --user-id has to be repeatable. A single-value flag would silently keep only
// the last id, and the batch add — the reason this endpoint exists — would add
// one member where the caller asked for several.
func TestBoardMembersAddUserIDIsRepeatable(t *testing.T) {
	flag := boardMembersAddCmd.Flags().Lookup("user-id")
	if flag == nil {
		t.Fatal("boards members add has no --user-id flag")
	}
	if got := flag.Value.Type(); got != "stringArray" {
		t.Errorf("--user-id is %s, want stringArray (repeatable)", got)
	}
}
