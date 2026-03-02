package cmd

import (
	"fmt"

	"lines/internal/model"

	"github.com/spf13/cobra"
)

var (
	listKind   string
	listPublic bool
)

var listCmd = &cobra.Command{
	Use:   "list <name>",
	Short: "List elements contained by a named element",
	Long: `List elements contained by a named element (package, struct, class,
module, etc). Accepts an FQName or name substring. Use --public to show only
exported elements and --kind to filter by element kind.

Valid kinds: function, method, class, struct, interface, trait, enum, impl, module`,
	Args: cobra.ExactArgs(1),
	RunE: runList,
}

func init() {
	listCmd.Flags().StringVar(&listKind, "kind", "", "Filter by element kind")
	listCmd.Flags().BoolVar(&listPublic, "public", false, "Show only public elements")
	rootCmd.AddCommand(listCmd)
}

func runList(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot()
	if err != nil {
		return err
	}

	store, err := openStore(root)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	results, err := store.GetContained(args[0])
	if err != nil {
		return fmt.Errorf("list contained: %w", err)
	}

	var filtered []model.Element
	for _, el := range results {
		if listKind != "" && el.Kind != listKind {
			continue
		}
		if listPublic && el.Visibility != model.VisPublic {
			continue
		}
		filtered = append(filtered, el)
	}

	if len(filtered) == 0 {
		logInfo("No elements found in %q", args[0])
		return nil
	}

	return output(filtered)
}
