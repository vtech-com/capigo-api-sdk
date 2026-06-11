package output

import (
	"fmt"
	"io"
)

// ListSummary describes the pagination state of a single list response so it
// can be rendered as a human- and agent-readable footer.
type ListSummary struct {
	// Shown is the number of rows rendered on this page.
	Shown int
	// Page, Limit, Total, HasMore mirror the API's pagination meta.
	Page    int
	Limit   int
	Total   int
	HasMore bool
	// HintAll advertises --all in the "more rows" hint (products list only).
	HintAll bool
}

// WriteListSummary prints a one-line pagination summary to w. It is meant for
// table mode and is written to the SAME stream as the table (stdout), not to
// stderr. The older nudge lived on stderr and only appeared when more pages
// existed, so an agent reading stdout would see a 20-row page and conclude the
// collection had 20 rows. Surfacing the total here — on every list, on the
// stream the agent actually reads — removes that trap.
//
// Examples:
//
//	Total: 43 · showing 20 (page 1/3) · more rows — use --page/--limit (max 100)
//	Total: 12 (all rows shown)
//	Total: 0 (no matching rows)
func WriteListSummary(w io.Writer, s ListSummary) {
	// Guard against endpoints that don't populate Total: never claim fewer
	// rows than we actually rendered.
	total := s.Total
	if total < s.Shown {
		total = s.Shown
	}

	if total == 0 {
		_, _ = fmt.Fprintln(w, "Total: 0 (no matching rows)")
		return
	}
	if s.Shown >= total {
		_, _ = fmt.Fprintf(w, "Total: %d (all rows shown)\n", total)
		return
	}

	line := fmt.Sprintf("Total: %d · showing %d", total, s.Shown)
	if s.Page > 0 {
		if pages := pageCount(total, s.Limit); pages > 0 {
			line += fmt.Sprintf(" (page %d/%d)", s.Page, pages)
		} else {
			line += fmt.Sprintf(" (page %d)", s.Page)
		}
	}

	hint := "--page/--limit (max 100)"
	if s.HintAll {
		hint += " or --all"
	}
	line += " · more rows — use " + hint

	_, _ = fmt.Fprintln(w, line)
}

// pageCount returns the number of pages for total rows at the given page size,
// or 0 when limit is unknown (so the caller can omit the "/N" suffix).
func pageCount(total, limit int) int {
	if limit <= 0 {
		return 0
	}
	return (total + limit - 1) / limit
}
