package cmd

import "github.com/spf13/cobra"

const recapText = `treelines recap

Purpose
  Deterministic codebase exploration for Go, Python, and Rust using Tree-sitter and SQLite.
  Use it as the first source of structure before selective file reads.

Core agent flow
  treelines init
  treelines index
  treelines overview
  treelines module-graph
  treelines list . --kind module
  treelines search <symbol-or-concept>
  treelines callees <fq_name>
  treelines uses <fq_name>
  treelines index

Overview
  treelines overview                  Compact first-pass map, default depth 2
  treelines overview --depth 1        Smaller stats and entry-point snapshot
  treelines overview --depth 3        Deeper map with key elements and entry relationships
  treelines --json overview           Agent-friendly structured output

Setup and lifecycle
  treelines init                      Create .treelines and initialize schema
  treelines index                     Full snapshot replacement
  treelines update                    Git commit-based incremental update
  treelines serve                     Filesystem watcher for ongoing local changes
  treelines stats                     Counts by language, kind, and edge type

Discovery
  treelines list . --kind module      Repo-wide module inventory
  treelines list <module>             Elements contained by a module
  treelines search <substring>        Symbol-oriented name/FQName search
  treelines element <name>            FQName, exact name, then substring lookup

Relationships
  treelines module-graph              Repo-wide module relationship summary
  treelines module-graph <module>     Per-module imports, functions, callers, callees
  treelines imports [module]          Internal import dependencies
  treelines exports [module]          Export surface
  treelines callees <fq_name>         Outgoing calls from an element
  treelines uses <fq_name>            Incoming callers of an element

SQL and automation
  treelines query --schema            Print schema and sample queries
  treelines query "<sql>"             Run raw SQL
  treelines query --file <path>       Run SQL from a file
  treelines --json <command>          Structured output for agents
  treelines --no-body <command>       Suppress element bodies where applicable

Agent rules
  Run treelines index before exploration and wait for it to finish.
  Start unknown codebases with treelines overview.
  Treat treelines as structural ground truth, then validate behavior with targeted file reads.
  Prefer index again for a fresh post-edit snapshot. Do not rely on update unless using a git commit-based workflow.
`

var recapCmd = &cobra.Command{
	Use:   "recap",
	Short: "Show a fuller treelines command recap",
	Long:  "Show a fuller agent-oriented recap of treelines workflows, commands, and examples.",
	Args:  cobra.NoArgs,
	Run: func(cmd *cobra.Command, args []string) {
		cmd.Print(recapText)
	},
}

func init() {
	rootCmd.AddCommand(recapCmd)
}
