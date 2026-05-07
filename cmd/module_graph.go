package cmd

import (
	"fmt"
	"text/tabwriter"

	"github.com/rpcarvs/treelines/internal/graph"
	"github.com/rpcarvs/treelines/internal/model"

	"github.com/spf13/cobra"
)

const defaultModuleGraphLimit = 10

var moduleGraphLimit int

var moduleGraphCmd = &cobra.Command{
	Use:   "module-graph [module]",
	Short: "Summarize module imports and call relationships",
	Long: `Show a compact module call-graph summary:
- internal imports for the module
- functions/methods in the module with incoming/outgoing call counts
- top outgoing callees from the module
- top incoming callers into the module

Without args, shows a repo-wide module overview.
With a module arg, shows per-module details.`,
	Args: cobra.MaximumNArgs(1),
	RunE: runModuleGraph,
}

func init() {
	moduleGraphCmd.Flags().IntVar(&moduleGraphLimit, "limit", defaultModuleGraphLimit, "Limit top callers/callees rows")
	rootCmd.AddCommand(moduleGraphCmd)
}

// runModuleGraph prints a module-level summary of imports and call relationships.
func runModuleGraph(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot()
	if err != nil {
		return err
	}
	store, err := openStore(root)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	limit := moduleGraphLimit
	if limit <= 0 {
		limit = defaultModuleGraphLimit
	}
	if len(args) == 0 {
		return outputModuleGraphOverview(store, limit)
	}

	module, err := resolveModuleElement(store, args[0])
	if err != nil {
		return err
	}
	if module.Kind != model.KindModule {
		return fmt.Errorf("%q is not a module", args[0])
	}

	imports, err := store.GetImportTargets(module.ID)
	if err != nil {
		return fmt.Errorf("get module imports: %w", err)
	}
	funcStats, err := store.RunSQL(`SELECT
	e.kind AS kind,
	e.fq_name AS fq_name,
	(SELECT COUNT(*) FROM edges c WHERE c.type = 'CALLS' AND c.from_id = e.id) AS outgoing_calls,
	(SELECT COUNT(*) FROM edges c WHERE c.type = 'CALLS' AND c.to_id = e.id) AS incoming_calls
FROM elements e
JOIN edges d ON d.from_id = e.id
WHERE d.type = 'DEFINED_IN'
	AND d.to_id = '` + module.ID + `'
	AND e.kind IN ('function', 'method')
ORDER BY e.kind, e.fq_name`)
	if err != nil {
		return fmt.Errorf("query module functions: %w", err)
	}
	topCallees, err := store.RunSQL(fmt.Sprintf(`SELECT
	t.fq_name AS callee,
	COUNT(*) AS calls,
	COUNT(DISTINCT s.id) AS caller_functions
FROM edges c
JOIN elements s ON s.id = c.from_id
JOIN elements t ON t.id = c.to_id
JOIN edges d ON d.from_id = s.id
WHERE c.type = 'CALLS'
	AND d.type = 'DEFINED_IN'
	AND d.to_id = '%s'
GROUP BY t.id
ORDER BY calls DESC, callee
LIMIT %d`, module.ID, limit))
	if err != nil {
		return fmt.Errorf("query top callees: %w", err)
	}
	topCallers, err := store.RunSQL(fmt.Sprintf(`SELECT
	s.fq_name AS caller,
	COUNT(*) AS calls,
	COUNT(DISTINCT t.id) AS target_functions
FROM edges c
JOIN elements s ON s.id = c.from_id
JOIN elements t ON t.id = c.to_id
JOIN edges d ON d.from_id = t.id
WHERE c.type = 'CALLS'
	AND d.type = 'DEFINED_IN'
	AND d.to_id = '%s'
GROUP BY s.id
ORDER BY calls DESC, caller
LIMIT %d`, module.ID, limit))
	if err != nil {
		return fmt.Errorf("query top callers: %w", err)
	}

	if flagJSON {
		return outputJSON(map[string]any{
			"module":          module,
			"imports":         imports,
			"functions":       funcStats,
			"top_callees":     topCallees,
			"top_callers":     topCallers,
			"top_limit":       limit,
			"imports_count":   len(imports),
			"functions_count": len(funcStats),
		})
	}

	_, _ = fmt.Fprintf(stdoutWriter(), "Module graph: %s (%s)\n", module.FQName, module.Language)
	_, _ = fmt.Fprintf(stdoutWriter(), "Path: %s\n", module.Path)
	_, _ = fmt.Fprintf(stdoutWriter(), "Imports: %d | Functions/Methods: %d\n", len(imports), len(funcStats))

	_, _ = fmt.Fprintln(stdoutWriter())
	_, _ = fmt.Fprintln(stdoutWriter(), "Imports")
	if len(imports) == 0 {
		_, _ = fmt.Fprintln(stdoutWriter(), "  none")
	} else {
		_ = printElementList(imports)
	}

	_, _ = fmt.Fprintln(stdoutWriter())
	_, _ = fmt.Fprintln(stdoutWriter(), "Functions and Methods")
	if len(funcStats) == 0 {
		_, _ = fmt.Fprintln(stdoutWriter(), "  none")
	} else {
		w := tabwriter.NewWriter(stdoutWriter(), 0, 0, 2, ' ', 0)
		_, _ = fmt.Fprintln(w, "kind\tfq_name\toutgoing_calls\tincoming_calls")
		for _, row := range funcStats {
			_, _ = fmt.Fprintf(w, "%v\t%v\t%v\t%v\n", row["kind"], row["fq_name"], row["outgoing_calls"], row["incoming_calls"])
		}
		_ = w.Flush()
	}

	_, _ = fmt.Fprintln(stdoutWriter())
	_, _ = fmt.Fprintf(stdoutWriter(), "Top Outgoing Callees (limit %d)\n", limit)
	if len(topCallees) == 0 {
		_, _ = fmt.Fprintln(stdoutWriter(), "  none")
	} else {
		_ = printTable(topCallees)
	}

	_, _ = fmt.Fprintln(stdoutWriter())
	_, _ = fmt.Fprintf(stdoutWriter(), "Top Incoming Callers (limit %d)\n", limit)
	if len(topCallers) == 0 {
		_, _ = fmt.Fprintln(stdoutWriter(), "  none")
	} else {
		_ = printTable(topCallers)
	}
	return nil
}

// outputModuleGraphOverview prints repo-wide module-level relationship summaries.
func outputModuleGraphOverview(store *graph.SQLiteStore, limit int) error {
	importRows, err := store.RunSQL(fmt.Sprintf(`SELECT
	m.fq_name AS module,
	m.language AS language,
	COUNT(*) AS imports
FROM edges e
JOIN elements m ON m.id = e.from_id
WHERE e.type = 'IMPORTS' AND m.kind = 'module'
GROUP BY m.id
ORDER BY imports DESC, module
LIMIT %d`, limit))
	if err != nil {
		return fmt.Errorf("query overview imports: %w", err)
	}
	outgoingRows, err := store.RunSQL(fmt.Sprintf(`SELECT
	m.fq_name AS module,
	m.language AS language,
	COUNT(*) AS outgoing_calls
FROM edges c
JOIN elements f ON f.id = c.from_id
JOIN edges d ON d.from_id = f.id AND d.type = 'DEFINED_IN'
JOIN elements m ON m.id = d.to_id
WHERE c.type = 'CALLS' AND m.kind = 'module'
GROUP BY m.id
ORDER BY outgoing_calls DESC, module
LIMIT %d`, limit))
	if err != nil {
		return fmt.Errorf("query overview outgoing calls: %w", err)
	}
	incomingRows, err := store.RunSQL(fmt.Sprintf(`SELECT
	m.fq_name AS module,
	m.language AS language,
	COUNT(*) AS incoming_calls
FROM edges c
JOIN elements f ON f.id = c.to_id
JOIN edges d ON d.from_id = f.id AND d.type = 'DEFINED_IN'
JOIN elements m ON m.id = d.to_id
WHERE c.type = 'CALLS' AND m.kind = 'module'
GROUP BY m.id
ORDER BY incoming_calls DESC, module
LIMIT %d`, limit))
	if err != nil {
		return fmt.Errorf("query overview incoming calls: %w", err)
	}

	if flagJSON {
		return outputJSON(map[string]any{
			"overview": map[string]any{
				"top_limit":         limit,
				"by_imports":        importRows,
				"by_outgoing_calls": outgoingRows,
				"by_incoming_calls": incomingRows,
			},
		})
	}

	_, _ = fmt.Fprintln(stdoutWriter(), "Module graph overview")
	_, _ = fmt.Fprintln(stdoutWriter())
	_, _ = fmt.Fprintf(stdoutWriter(), "Top Modules by Imports (limit %d)\n", limit)
	if len(importRows) == 0 {
		_, _ = fmt.Fprintln(stdoutWriter(), "  none")
	} else {
		_ = printTable(importRows)
	}
	_, _ = fmt.Fprintln(stdoutWriter())
	_, _ = fmt.Fprintf(stdoutWriter(), "Top Modules by Outgoing Calls (limit %d)\n", limit)
	if len(outgoingRows) == 0 {
		_, _ = fmt.Fprintln(stdoutWriter(), "  none")
	} else {
		_ = printTable(outgoingRows)
	}
	_, _ = fmt.Fprintln(stdoutWriter())
	_, _ = fmt.Fprintf(stdoutWriter(), "Top Modules by Incoming Calls (limit %d)\n", limit)
	if len(incomingRows) == 0 {
		_, _ = fmt.Fprintln(stdoutWriter(), "  none")
	} else {
		_ = printTable(incomingRows)
	}
	return nil
}
