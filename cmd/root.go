package cmd

import (
	"context"
	"os"

	"github.com/charmbracelet/fang"
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
	Use:   "treelines",
	Short: "Code intelligence powered by Tree-sitter and graph queries",
	Long: `Treelines parses codebases using Tree-sitter, extracts code elements
(functions, methods, classes, structs, interfaces, traits, enums, impl blocks, modules), and stores them in a
local SQLite database for queryable code intelligence.

Treelines commands are scoped to the current Git repository root and can be run
from any subdirectory inside that repository.

Use symbol commands (element/search/list/uses/callees) for structural graph work.
Use overview for a compact first-pass map of an unknown codebase.
Use imports for internal module dependency surface.
Use exports for language-aware module export surface.
Use onboard or recap for agent workflow reminders.`,
}

// Execute runs the root cobra command through Fang and exits on error.
func Execute(version string) {
	options := []fang.Option{fang.WithoutManpage()}
	if version != "" {
		options = append(options, fang.WithVersion(version))
	}

	if err := fang.Execute(context.Background(), rootCmd, options...); err != nil {
		os.Exit(1)
	}
}

func init() {
	rootCmd.SetOut(os.Stdout)
	rootCmd.SetErr(os.Stderr)
	rootCmd.PersistentFlags().StringVar(&flagDB, "db", "", "Database path (default: <git-root>/.treelines/codestore.db)")
	rootCmd.PersistentFlags().BoolVar(&flagVerbose, "verbose", false, "Enable verbose output")
	rootCmd.PersistentFlags().BoolVar(&flagQuiet, "quiet", false, "Suppress non-essential output")
	rootCmd.PersistentFlags().BoolVar(&flagNoBody, "no-body", false, "Strip body field from output")
	rootCmd.PersistentFlags().BoolVar(&flagJSON, "json", false, "Output as JSON")
}
