package cmd

import (
	"fmt"
	"path/filepath"

	"lines/internal/extractor"
	"lines/internal/graph"
	"lines/internal/model"
	"lines/internal/parser"

	"github.com/spf13/cobra"
)

var exportsSource bool

var exportsCmd = &cobra.Command{
	Use:   "exports [module]",
	Short: "Show Python __all__ export surface",
	Long: `Show Python export surface derived from static __all__ assignments.
This is the authoritative command for Python package export surface.

Without arguments, lists modules with resolved export counts.
With a module name/FQName, lists each exported symbol and resolution status.
Use --source to include __all__ assignment location (path and line).`,
	Args: cobra.MaximumNArgs(1),
	RunE: runExports,
}

func init() {
	exportsCmd.Flags().BoolVar(&exportsSource, "source", false, "Include __all__ assignment location")
	rootCmd.AddCommand(exportsCmd)
}

// runExports reports module-level export surface information.
func runExports(cmd *cobra.Command, args []string) error {
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
	COUNT(*) AS exports
FROM edges e
JOIN elements src ON src.id = e.from_id
WHERE e.type = 'EXPORTS'
GROUP BY src.id
ORDER BY exports DESC, module`)
		if err != nil {
			return fmt.Errorf("list exports: %w", err)
		}
		if len(rows) == 0 {
			logInfo("No exports found")
			return nil
		}
		return output(rows)
	}

	module, err := resolveModuleElement(store, args[0])
	if err != nil {
		return err
	}
	if module.Language != model.LangPython || module.Kind != model.KindModule {
		return fmt.Errorf("%q is not a python module", args[0])
	}

	names, line, hasLine, err := parseModuleAll(root, module)
	if err != nil {
		return err
	}
	if len(names) == 0 {
		logInfo("No static __all__ found for %q", module.FQName)
		return nil
	}

	importTargets, err := store.GetImportTargets(module.ID)
	if err != nil {
		return fmt.Errorf("get import targets: %w", err)
	}
	importByName := make(map[string]string)
	for _, el := range importTargets {
		if _, exists := importByName[el.Name]; !exists {
			importByName[el.Name] = el.FQName
		}
	}

	rows := make([]map[string]any, 0, len(names))
	for _, name := range names {
		resolvedFQName := ""
		status := "unresolved"

		localFQName := module.FQName + "." + name
		if target, err := store.GetElement(localFQName); err == nil && target != nil {
			resolvedFQName = target.FQName
			status = "resolved"
		} else if targetFQ, ok := importByName[name]; ok {
			resolvedFQName = targetFQ
			status = "resolved"
		}

		row := map[string]any{
			"module":            module.FQName,
			"exported_name":     name,
			"exported_fq_name":  resolvedFQName,
			"resolution_status": status,
		}
		if exportsSource {
			row["path"] = module.Path
			if hasLine {
				row["line"] = line
			} else {
				row["line"] = nil
			}
		}
		rows = append(rows, row)
	}
	return output(rows)
}

// resolveModuleElement finds a module by FQName first, then exact short name.
func resolveModuleElement(store *graph.SQLiteStore, query string) (*model.Element, error) {
	if el, err := store.GetElement(query); err != nil {
		return nil, fmt.Errorf("resolve module by fq_name: %w", err)
	} else if el != nil && el.Kind == model.KindModule {
		return el, nil
	}

	exact, err := store.GetElementByExactName(query)
	if err != nil {
		return nil, fmt.Errorf("resolve module by name: %w", err)
	}
	var modules []model.Element
	for _, el := range exact {
		if el.Kind == model.KindModule {
			modules = append(modules, el)
		}
	}
	if len(modules) == 0 {
		return nil, fmt.Errorf("module %q not found", query)
	}
	if len(modules) > 1 {
		return nil, fmt.Errorf("module %q is ambiguous; use fq_name", query)
	}
	return &modules[0], nil
}

// parseModuleAll extracts static __all__ names and assignment line from module source.
func parseModuleAll(root string, module *model.Element) ([]string, int, bool, error) {
	p := parser.NewParser()
	defer p.Close()
	absPath := filepath.Join(root, module.Path)
	result, err := p.ParseFile(absPath, module.Path, module.Language)
	if err != nil {
		return nil, 0, false, fmt.Errorf("parse module: %w", err)
	}
	defer result.Tree.Close()

	names, line, hasLine, err := extractor.ExtractPythonAll(result)
	if err != nil {
		return nil, 0, false, fmt.Errorf("extract __all__: %w", err)
	}
	return names, line, hasLine, nil
}
