package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var elementCmd = &cobra.Command{
	Use:   "element <fq_name>",
	Short: "Look up a code element by fully qualified name",
	Args:  cobra.ExactArgs(1),
	RunE:  runElement,
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
	defer store.Close()

	name := args[0]

	el, err := store.GetElement(name)
	if err != nil {
		return fmt.Errorf("get element: %w", err)
	}
	if el != nil {
		return output(el)
	}

	elements, err := store.GetElementsByName(name)
	if err != nil {
		return fmt.Errorf("search elements: %w", err)
	}
	if len(elements) == 0 {
		return fmt.Errorf("no element found matching %q", name)
	}

	return output(elements)
}
