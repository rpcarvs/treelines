package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var usesCmd = &cobra.Command{
	Use:   "uses <fq_name>",
	Short: "List callers of a code element",
	Long: `List all functions and methods that call the given element.
Requires a fully qualified name (e.g., "graph.SQLiteStore.Open").
Use "lines search" to discover FQNames.`,
	Args: cobra.ExactArgs(1),
	RunE: runUses,
}

func init() {
	rootCmd.AddCommand(usesCmd)
}

func runUses(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot()
	if err != nil {
		return err
	}

	store, err := openStore(root)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	results, err := store.GetCallers(args[0])
	if err != nil {
		return fmt.Errorf("get callers: %w", err)
	}

	if len(results) == 0 {
		logInfo("No callers found for %q", args[0])
		return nil
	}

	return output(results)
}
