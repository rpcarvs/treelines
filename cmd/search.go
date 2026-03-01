package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var searchCmd = &cobra.Command{
	Use:   "search <substring>",
	Short: "Search for code elements by name substring",
	Args:  cobra.ExactArgs(1),
	RunE:  runSearch,
}

func init() {
	rootCmd.AddCommand(searchCmd)
}

func runSearch(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot()
	if err != nil {
		return err
	}

	store, err := openStore(root)
	if err != nil {
		return err
	}
	defer store.Close()

	results, err := store.Search(args[0])
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	if len(results) == 0 {
		logInfo("No results found for %q", args[0])
		return nil
	}

	return output(results)
}
