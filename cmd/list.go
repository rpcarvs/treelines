package cmd

import (
	"fmt"

	"github.com/rpcarvs/treelines/internal/graph"
	"github.com/rpcarvs/treelines/internal/model"

	"github.com/spf13/cobra"
)

var (
	listKind   string
	listPublic bool
)

var listCmd = &cobra.Command{
	Use:   "list <name|.|*>",
	Short: "List elements contained by a named element",
	Long: `List elements contained by a named element (package, struct, class,
module, etc). Accepts an FQName or name substring. Use --public to show only
exported elements and --kind to filter by element kind.
Use "." or "*" as repo-wide scope.

When --kind module is set and containment has no module children, list falls
back to module-prefix matching (for example "taggers" -> "taggers.*").

Valid kinds: function, method, class, struct, interface, trait, enum, impl, module`,
	Args: cobra.ExactArgs(1),
	RunE: runList,
}

func init() {
	listCmd.Flags().StringVar(&listKind, "kind", "", "Filter by element kind")
	listCmd.Flags().BoolVar(&listPublic, "public", false, "Show only public elements")
	rootCmd.AddCommand(listCmd)
}

// runList lists elements contained by a named parent element.
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

	if args[0] == "." || args[0] == "*" {
		all, err := store.GetAllElements()
		if err != nil {
			return fmt.Errorf("list global elements: %w", err)
		}
		var filtered []model.Element
		for _, el := range all {
			if listKind != "" && el.Kind != listKind {
				continue
			}
			if listPublic && el.Visibility != model.VisPublic {
				continue
			}
			filtered = append(filtered, el)
		}
		if len(filtered) == 0 {
			logInfo("No elements found in repo scope")
			return nil
		}
		return output(filtered)
	}

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

	if len(filtered) == 0 && listKind == model.KindModule {
		prefixModules, err := listModulePrefixFallback(store, args[0])
		if err != nil {
			return err
		}
		for _, el := range prefixModules {
			if listPublic && el.Visibility != model.VisPublic {
				continue
			}
			filtered = append(filtered, el)
		}
	}

	if len(filtered) == 0 {
		logInfo("No elements found in %q", args[0])
		return nil
	}

	return output(filtered)
}

// listModulePrefixFallback returns child modules by fq_name prefix for module queries.
func listModulePrefixFallback(store *graph.SQLiteStore, query string) ([]model.Element, error) {
	module, err := resolveModuleElement(store, query)
	if err != nil {
		return nil, nil
	}
	modules, err := store.GetModulesByPrefix(module.FQName)
	if err != nil {
		return nil, fmt.Errorf("list module prefix: %w", err)
	}
	return modules, nil
}
