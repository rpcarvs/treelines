package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	flagFormat  string
	flagDB      string
	flagVerbose bool
	flagQuiet   bool
)

var rootCmd = &cobra.Command{
	Use:   "lines",
	Short: "Code intelligence powered by Tree-sitter and graph queries",
	Long: `Treelines parses codebases using Tree-sitter, extracts code elements
(functions, classes, structs, traits, interfaces), and stores them in a
LadybugDB graph database for queryable code intelligence.`,
}

func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagFormat, "format", "text", "Output format: text or json")
	rootCmd.PersistentFlags().StringVar(&flagDB, "db", "", "Database path (default: .treelines/db)")
	rootCmd.PersistentFlags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-essential output")
}
