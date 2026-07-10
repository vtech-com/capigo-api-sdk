// Package cmd — help_footer.go
//
// Every help page ends with the build it came from. Help text ships inside the
// binary, so a page can never disagree with the binary that printed it — but it
// can silently describe an older surface than the one deployed. The footer lets
// a reader tell which build they are reading, and points at the topic that
// explains what that implies.
//
// The footer is written by renderHelp (help_render.go), which is the only
// thing that prints a help page.
package cmd

import "fmt"

// helpFooter renders the trailing line of every help page. Date is empty or
// "unknown" for local `go build` output, where a build timestamp would be noise.
func helpFooter(v, date string) string {
	if date == "" || date == "unknown" {
		return fmt.Sprintf("\ncapigo %s · capigo help versioning\n", v)
	}
	return fmt.Sprintf("\ncapigo %s (built %s) · capigo help versioning\n", v, date)
}
