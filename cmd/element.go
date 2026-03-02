package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var elementCmd = &cobra.Command{
	Use:   "element <name>",
	Short: "Look up a code element by fully qualified name",
	Long: `Look up a code element using a three-step search:

  1. Exact FQName match (e.g., "graph.SQLiteStore.Open")
  2. Exact short name match (e.g., "Open")
  3. Substring match (e.g., "Store")

FQName formats vary by language:
  Go:     package.Function, package.Type.Method
  Python: module.Class.method
  Rust:   crate::module::Type::method`,
	Args: cobra.ExactArgs(1),
	RunE: runElement,
}

func init() {
	rootCmd.AddCommand(elementCmd)
}

func runElement(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot()
	if err != nil {
		return err
	}

	store, err := openStore(root)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	name := args[0]

	// Step 1: Exact FQName match
	el, err := store.GetElement(name)
	if err != nil {
		return fmt.Errorf("get element: %w", err)
	}
	if el != nil {
		return output(el)
	}

	// Step 2: Exact name match
	exact, err := store.GetElementByExactName(name)
	if err != nil {
		return fmt.Errorf("exact name search: %w", err)
	}
	if len(exact) > 0 {
		return output(exact)
	}

	// Step 3: Substring search
	elements, err := store.GetElementsByName(name)
	if err != nil {
		return fmt.Errorf("search elements: %w", err)
	}
	if len(elements) == 0 {
		return fmt.Errorf("no element found matching %q", name)
	}

	return output(elements)
}
