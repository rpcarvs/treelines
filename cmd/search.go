package cmd

import (
	"fmt"

	"lines/internal/model"

	"github.com/spf13/cobra"
)

var searchKind string

var searchCmd = &cobra.Command{
	Use:   "search <substring>",
	Short: "Search for code elements by name substring",
	Long: `Search for code elements whose name or FQName contains the given
substring. Use --kind to narrow results to a specific element kind.

Valid kinds: function, method, class, struct, interface, trait, enum, impl, module`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

func init() {
	searchCmd.Flags().StringVar(&searchKind, "kind", "", "Filter by element kind")
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
	defer func() { _ = store.Close() }()

	results, err := store.Search(args[0])
	if err != nil {
		return fmt.Errorf("search: %w", err)
	}

	if searchKind != "" {
		var filtered []model.Element
		for _, el := range results {
			if el.Kind == searchKind {
				filtered = append(filtered, el)
			}
		}
		results = filtered
	}

	if len(results) == 0 {
		logInfo("No results found for %q", args[0])
		return nil
	}

	return output(results)
}
