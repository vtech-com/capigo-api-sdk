package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// paginationHint is the exact string list commands print when more results
// exist. Any file that prints it is promising the user --page/--limit.
const paginationHint = "Use --page / --limit to paginate"

// paginatedListCommands are every list command that prints the
// "Use --page / --limit to paginate." hint. Because they advertise those
// flags to the user, each one MUST register --page and --limit. This guards
// against the boards list regression where the hint was printed but the flags
// were never registered (the command rejected --page with "unknown flag").
//
// When adding a new list command that paginates, add it here. The
// `make check-hints` target catches files that print the hint without the
// flags even if they are forgotten here.
var paginatedListCommands = map[string]*cobra.Command{
	"boards list":        boardsListCmd,
	"members list":       membersListCmd,
	"brands list":        brandsListCmd,
	"categories list":    categoriesListCmd,
	"product-types list": productTypesListCmd,
	"products list":      productsListCmd,
	"tasks list":         tasksListCmd,
	"units list":         unitsListCmd,
	"variants list":      variantsListCmd,
}

func TestPaginatedListCommandsRegisterPaginationFlags(t *testing.T) {
	for name, cmd := range paginatedListCommands {
		for _, flag := range []string{"page", "limit"} {
			if cmd.Flags().Lookup(flag) == nil {
				t.Errorf("%q advertises --page/--limit but does not register --%s", name, flag)
			}
		}
	}
}

// TestPaginationHintImpliesFlags scans every command source file: if a file
// prints the pagination hint, it must also register the --page and --limit
// flags it advertises. This is the dynamic backstop that catches a new list
// command that prints the hint but is forgotten from paginatedListCommands
// above — the exact way the boards list bug shipped.
func TestPaginationHintImpliesFlags(t *testing.T) {
	files, err := filepath.Glob("*.go")
	if err != nil {
		t.Fatalf("glob cmd sources: %v", err)
	}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		src, err := os.ReadFile(f)
		if err != nil {
			t.Fatalf("read %s: %v", f, err)
		}
		body := string(src)
		if !strings.Contains(body, paginationHint) {
			continue
		}
		for _, flag := range []string{`"page"`, `"limit"`} {
			if !strings.Contains(body, flag) {
				t.Errorf("%s prints the pagination hint but never registers a %s flag", f, flag)
			}
		}
	}
}
