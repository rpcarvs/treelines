package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var calleesCmd = &cobra.Command{
	Use:   "callees <fq_name>",
	Short: "List functions called by a code element",
	Long: `List all functions and methods that the given element calls.
Requires a fully qualified name (e.g., "cmd.runIndex").
Use "lines search" to discover FQNames.`,
	Args: cobra.ExactArgs(1),
	RunE: runCallees,
}

func init() {
	rootCmd.AddCommand(calleesCmd)
}

func runCallees(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot()
	if err != nil {
		return err
	}

	store, err := openStore(root)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	results, err := store.GetCallees(args[0])
	if err != nil {
		return fmt.Errorf("get callees: %w", err)
	}

	if len(results) == 0 {
		logInfo("No callees found for %q", args[0])
		return nil
	}

	return output(results)
}
