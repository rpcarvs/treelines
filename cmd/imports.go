package cmd

import (
	"fmt"

	"lines/internal/model"

	"github.com/spf13/cobra"
)

var importsCmd = &cobra.Command{
	Use:   "imports [module]",
	Short: "Show internal import dependencies",
	Long: `Show internal import dependencies from IMPORTS edges.
This command is module-level and does not show per-function calls.

Without arguments, lists modules with import counts.
With a module name/FQName, lists imported internal modules/symbols.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runImports,
}

func init() {
	rootCmd.AddCommand(importsCmd)
}

// runImports reports internal import dependencies.
func runImports(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot()
	if err != nil {
		return err
	}
	store, err := openStore(root)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	if len(args) == 0 {
		rows, err := store.RunSQL(`SELECT
	src.fq_name AS module,
	src.path AS path,
	COUNT(*) AS imports
FROM edges e
JOIN elements src ON src.id = e.from_id
WHERE e.type = 'IMPORTS'
GROUP BY src.id
ORDER BY imports DESC, module`)
		if err != nil {
			return fmt.Errorf("list imports: %w", err)
		}
		if len(rows) == 0 {
			logInfo("No imports found")
			return nil
		}
		return output(rows)
	}

	module, err := resolveModuleElement(store, args[0])
	if err != nil {
		return err
	}
	if module.Kind != model.KindModule {
		return fmt.Errorf("%q is not a module", args[0])
	}
	targets, err := store.GetImportTargets(module.ID)
	if err != nil {
		return fmt.Errorf("get imports: %w", err)
	}
	if len(targets) == 0 {
		logInfo("No imports found for %q", module.FQName)
		return nil
	}
	return output(targets)
}
