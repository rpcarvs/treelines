package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var (
	flagDB      string
	flagVerbose bool
	flagQuiet   bool
	flagNoBody  bool
	flagJSON    bool
)

var rootCmd = &cobra.Command{
	Use:   "lines",
	Short: "Code intelligence powered by Tree-sitter and graph queries",
	Long: `Treelines parses codebases using Tree-sitter, extracts code elements
(functions, methods, classes, structs, interfaces, traits, enums, impl blocks, modules), and stores them in a
local SQLite database for queryable code intelligence.

Use symbol commands (element/search/list/uses/callees) for structural graph work.
Use imports for internal module dependency surface.
Use exports for language-aware module export surface.`,
}

// Execute runs the root cobra command and exits on error.
func Execute() {
	err := rootCmd.Execute()
	if err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.PersistentFlags().StringVar(&flagDB, "db", "", "Database path (default: .treelines/codestore.db)")
	rootCmd.PersistentFlags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&flagNoBody, "no-body", false, "Strip body field from output")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output as JSON")
}
