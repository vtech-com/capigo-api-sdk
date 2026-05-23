package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
	"github.com/vtech-com/capigo-api-sdk/internal/version"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print the capigo version",
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
