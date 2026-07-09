package cmd

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// paginationMarker is the call a list command makes to build its meta. A file
// that calls it emits page/limit/total/has_more, so it advertises pagination
// and MUST register --page and --limit.
const paginationMarker = "listMeta("

// paginatedListCommands are every list command that reports pagination in its
// meta. Because they advertise --page/--limit to the user, each one MUST
// register those flags. This guards
// against the boards list regression where the hint was printed but the flags
// were never registered (the command rejected --page with "unknown flag").
//
// When adding a new list command that paginates, add it here. The
// `make check-hints` target catches files that report pagination without the
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
// reports pagination in its meta, it must also register the --page and --limit
// flags that meta invites the caller to use. This is the dynamic backstop that
// catches a new list command forgotten from paginatedListCommands above — the
// exact way the boards list bug shipped.
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
		// The file that declares listMeta is not a command that calls it.
		if strings.Contains(body, "func listMeta(") {
			continue
		}
		if !strings.Contains(body, paginationMarker) {
			continue
		}
		for _, flag := range []string{`"page"`, `"limit"`} {
			if !strings.Contains(body, flag) {
				t.Errorf("%s reports pagination in meta but never registers a %s flag", f, flag)
			}
		}
	}
}
