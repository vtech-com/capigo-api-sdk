// Package cmd — help_footer.go
//
// Every help page ends with the build it came from. Help text ships inside the
// binary, so a page can never disagree with the binary that printed it — but it
// can silently describe an older surface than the one deployed. The footer lets
// a reader tell which build they are reading, and points at the topic that
// explains what that implies.
package cmd

import (
	"fmt"

	"github.com/vtech-com/capigo-api-sdk/internal/version"
)

// baseHelpTemplate is cobra's default help template, reproduced verbatim so
// that the appended footer is the only difference from stock behaviour.
const baseHelpTemplate = `{{with (or .Long .Short)}}{{. | trimTrailingWhitespaces}}

{{end}}{{if or .Runnable .HasSubCommands}}{{.UsageString}}{{end}}`

// helpFooter renders the trailing line of every help page. Date is empty or
// "unknown" for local `go build` output, where a build timestamp would be noise.
func helpFooter(v, date string) string {
	if date == "" || date == "unknown" {
		return fmt.Sprintf("\ncapigo %s · capigo help versioning\n", v)
	}
	return fmt.Sprintf("\ncapigo %s (built %s) · capigo help versioning\n", v, date)
}

func init() {
	// Cobra resolves a command's help template by walking up to its parent when
	// it has none of its own, so setting this on the root covers every command
	// and every help topic.
	rootCmd.SetHelpTemplate(baseHelpTemplate + helpFooter(version.Version, version.Date))
}
