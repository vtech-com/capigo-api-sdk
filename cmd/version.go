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
  pages you read describe this build and no other.

INPUT
  (no flags)

OUTPUT
  Three lines: the version, the commit, and the build date.

  This command ignores --output: it prints the same text in every mode.

EXAMPLES
  capigo version

SEE ALSO
  capigo help versioning   how this CLI relates to the API it calls`,
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
