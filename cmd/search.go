package cmd

import (
	"fmt"
	"strings"

	"github.com/rpcarvs/treelines/internal/model"

	"github.com/spf13/cobra"
)

var searchKind string

var searchCmd = &cobra.Command{
	Use:   "search <substring>",
	Short: "Search for code elements by name substring",
	Long: `Search for code elements whose name or FQName contains the given
substring. Use --kind to narrow results to a specific element kind.

This command is symbol-oriented. For module export surface, use "treelines exports".

Compatibility fallback: searching for "__all__" also reports modules with
static EXPORTS edges derived from Python __all__ assignments.

Valid kinds: function, method, class, struct, interface, trait, enum, impl, module`,
	Args: cobra.ExactArgs(1),
	RunE: runSearch,
}

func init() {
	searchCmd.Flags().StringVar(&searchKind, "kind", "", "Filter by element kind")
	rootCmd.AddCommand(searchCmd)
}

// runSearch searches for elements by name substring with optional kind filter.
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
		if searchKind == "" && strings.Contains(strings.ToLower(args[0]), "__all__") {
			exportRows, err := store.RunSQL(`SELECT
	src.fq_name AS module,
	src.path AS path,
	COUNT(*) AS exports
FROM edges e
JOIN elements src ON src.id = e.from_id
WHERE e.type = 'EXPORTS'
GROUP BY src.id
ORDER BY exports DESC, module`)
			if err != nil {
				return fmt.Errorf("search __all__ export surface: %w", err)
			}
			if len(exportRows) > 0 {
				return output(exportRows)
			}
		}
		logInfo("No results found for %q", args[0])
		return nil
	}

	return output(results)
}
