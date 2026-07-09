package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the capigo version",
	Long: `Print the build of this CLI.

PURPOSE
  Report which binary is running. Help text ships inside the binary, so the
  page you are reading describes this build and no other.

USAGE
  capigo version

FLAGS
  (none)

OUTPUT
  Three lines: the version, the commit, and the build date.

      capigo 0.16.0
      commit: 4a1f9c2
      built:  2026-06-18T10:03:00Z

  This command ignores --output: it prints the same text in every mode.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		fmt.Printf("capigo %s\n", version.Version)
		fmt.Printf("commit: %s\n", version.Commit)
		fmt.Printf("built:  %s\n", version.Date)
		return nil
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
