package cmd

import (
	"testing"

	"github.com/spf13/cobra"
)

func TestShouldHintJSON(t *testing.T) {
	cases := []struct {
		name          string
		mode          string
		outputChanged bool
		stdoutIsTTY   bool
		hintsDisabled bool
		want          bool
	}{
		{"captured default table -> hint", "table", false, false, false, true},
		{"interactive terminal -> no hint", "table", false, true, false, false},
		{"explicit --output table -> no hint", "table", true, false, false, false},
		{"json mode -> no hint", "json", false, false, false, false},
		{"quiet mode -> no hint", "quiet", false, false, false, false},
		{"hints silenced -> no hint", "table", false, false, true, false},
		{"empty mode treated as non-table -> no hint", "", false, false, false, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := shouldHintJSON(c.mode, c.outputChanged, c.stdoutIsTTY, c.hintsDisabled)
			if got != c.want {
				t.Errorf("shouldHintJSON(%q, changed=%v, tty=%v, noHints=%v) = %v, want %v",
					c.mode, c.outputChanged, c.stdoutIsTTY, c.hintsDisabled, got, c.want)
			}
		})
	}
}

func TestCmdExemptFromJSONHint(t *testing.T) {
	root := &cobra.Command{Use: "capigo"}
	version := &cobra.Command{Use: "version"}
	config := &cobra.Command{Use: "config"}
	configGet := &cobra.Command{Use: "get"}
	config.AddCommand(configGet)
	products := &cobra.Command{Use: "products"}
	productsList := &cobra.Command{Use: "list"}
	products.AddCommand(productsList)
	root.AddCommand(version, config, products)

	cases := []struct {
		name string
		cmd  *cobra.Command
		want bool
	}{
		{"version is exempt", version, true},
		{"config group is exempt", config, true},
		{"config get (subcommand) is exempt via ancestor", configGet, true},
		{"products list is not exempt", productsList, false},
		{"root is not exempt", root, false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := cmdExemptFromJSONHint(c.cmd); got != c.want {
				t.Errorf("cmdExemptFromJSONHint(%q) = %v, want %v", c.cmd.Name(), got, c.want)
			}
		})
	}
}
