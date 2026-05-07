package cmd

import (
	"fmt"

	"github.com/rpcarvs/treelines/internal/graph"

	"github.com/spf13/cobra"
)

const (
	defaultOverviewDepth = 2
	defaultOverviewLimit = 8
)

var (
	overviewDepth int
	overviewLimit int
)

var overviewCmd = &cobra.Command{
	Use:   "overview",
	Short: "Show a compact first-pass codebase overview",
	Long: `Show an agent-focused overview of the indexed codebase.

The overview is deterministic and read-only. It aggregates existing graph data
into a compact orientation report before targeted exploration or file reads.

Depth levels:
  1: stats, languages, top modules, entry points
  2: depth 1 plus dependency sketch, exports, connected modules, next commands
  3: depth 2 plus key elements and entry point relationships`,
	Args: cobra.NoArgs,
	RunE: runOverview,
}

func init() {
	overviewCmd.Flags().IntVar(&overviewDepth, "depth", defaultOverviewDepth, "Overview depth: 1, 2, or 3")
	overviewCmd.Flags().IntVar(&overviewLimit, "limit", defaultOverviewLimit, "Limit rows per overview section")
	rootCmd.AddCommand(overviewCmd)
}

// runOverview prints a deterministic orientation report for agents and humans.
func runOverview(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot()
	if err != nil {
		return err
	}
	store, err := openStore(root)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	depth, err := normalizeOverviewDepth(overviewDepth)
	if err != nil {
		return err
	}
	limit := normalizePositiveLimit(overviewLimit, defaultOverviewLimit)

	data, err := buildOverview(store, depth, limit)
	if err != nil {
		return err
	}
	if flagJSON {
		return outputJSON(data)
	}
	return printOverview(data)
}

// overviewData contains the compact report sections emitted by overview.
type overviewData struct {
	Depth              int              `json:"depth"`
	Limit              int              `json:"limit"`
	Project            map[string]any   `json:"project"`
	ElementsByKind     []map[string]any `json:"elements_by_kind"`
	ElementsByLanguage []map[string]any `json:"elements_by_language"`
	EdgesByType        []map[string]any `json:"edges_by_type"`
	TopModules         []map[string]any `json:"top_modules"`
	EntryPoints        []map[string]any `json:"entry_points"`
	DependencySketch   []map[string]any `json:"dependency_sketch,omitempty"`
	ExportSurface      []map[string]any `json:"export_surface,omitempty"`
	OutgoingModules    []map[string]any `json:"top_outgoing_modules,omitempty"`
	IncomingModules    []map[string]any `json:"top_incoming_modules,omitempty"`
	KeyElements        []map[string]any `json:"key_elements,omitempty"`
	EntryRelationships []map[string]any `json:"entry_relationships,omitempty"`
	SuggestedCommands  []string         `json:"suggested_commands,omitempty"`
}

// buildOverview collects overview sections from the existing graph database.
func buildOverview(store *graph.SQLiteStore, depth int, limit int) (*overviewData, error) {
	counts, err := queryOverviewCounts(store)
	if err != nil {
		return nil, err
	}
	topModules, err := queryOverviewTopModules(store, limit)
	if err != nil {
		return nil, err
	}
	entryPoints, err := queryOverviewEntryPoints(store, limit)
	if err != nil {
		return nil, err
	}

	data := &overviewData{
		Depth:              depth,
		Limit:              limit,
		Project:            counts.project,
		ElementsByKind:     counts.byKind,
		ElementsByLanguage: counts.byLanguage,
		EdgesByType:        counts.byEdge,
		TopModules:         topModules,
		EntryPoints:        entryPoints,
	}

	if depth >= 2 {
		if data.DependencySketch, err = queryOverviewDependencySketch(store, limit); err != nil {
			return nil, err
		}
		if data.ExportSurface, err = queryOverviewExportSurface(store, limit); err != nil {
			return nil, err
		}
		if data.OutgoingModules, err = queryOverviewModuleCalls(store, "from_id", limit); err != nil {
			return nil, err
		}
		if data.IncomingModules, err = queryOverviewModuleCalls(store, "to_id", limit); err != nil {
			return nil, err
		}
		data.SuggestedCommands = suggestOverviewCommands(topModules, entryPoints)
	}

	if depth >= 3 {
		if data.KeyElements, err = queryOverviewKeyElements(store, limit); err != nil {
			return nil, err
		}
		if data.EntryRelationships, err = queryOverviewEntryRelationships(store, limit); err != nil {
			return nil, err
		}
	}

	return data, nil
}

type overviewCounts struct {
	project    map[string]any
	byKind     []map[string]any
	byLanguage []map[string]any
	byEdge     []map[string]any
}

// queryOverviewCounts returns count sections used by the overview header.
func queryOverviewCounts(store *graph.SQLiteStore) (*overviewCounts, error) {
	rows, err := store.RunSQL(`
WITH
  by_kind AS (
    SELECT 'kind' AS section, kind AS label, COUNT(*) AS count
    FROM elements
    GROUP BY kind
  ),
  by_language AS (
    SELECT 'language' AS section, language AS label, COUNT(*) AS count
    FROM elements
    GROUP BY language
  ),
  by_edge AS (
    SELECT 'edge' AS section, type AS label, COUNT(*) AS count
    FROM edges
    GROUP BY type
  ),
  totals AS (
    SELECT 'project' AS section, 'total_elements' AS label, COUNT(*) AS count
    FROM elements
    UNION ALL
    SELECT 'project' AS section, 'total_edges' AS label, COUNT(*) AS count
    FROM edges
    UNION ALL
    SELECT 'project' AS section, 'source_files' AS label, COUNT(DISTINCT path) AS count
    FROM elements
    WHERE path != '' AND start_line > 0
  )
SELECT section, label, count FROM by_kind
UNION ALL
SELECT section, label, count FROM by_language
UNION ALL
SELECT section, label, count FROM by_edge
UNION ALL
SELECT section, label, count FROM totals
ORDER BY section, count DESC, label`)
	if err != nil {
		return nil, fmt.Errorf("query overview counts: %w", err)
	}

	result := &overviewCounts{
		project: map[string]any{},
	}
	for _, row := range rows {
		section, _ := row["section"].(string)
		label, _ := row["label"].(string)
		count := toInt64(row["count"])
		switch section {
		case "kind":
			result.byKind = append(result.byKind, map[string]any{"kind": label, "count": count})
		case "language":
			result.byLanguage = append(result.byLanguage, map[string]any{"language": label, "count": count})
		case "edge":
			result.byEdge = append(result.byEdge, map[string]any{"type": label, "count": count})
		case "project":
			result.project[label] = count
		}
	}
	return result, nil
}

// queryOverviewTopModules returns modules ranked by local definitions.
func queryOverviewTopModules(store *graph.SQLiteStore, limit int) ([]map[string]any, error) {
	rows, err := store.RunSQL(fmt.Sprintf(`SELECT
	m.language AS language,
	m.fq_name AS module,
	m.path AS path,
	COUNT(child.id) AS elements,
	COALESCE(SUM(child.loc), 0) AS loc
FROM elements m
LEFT JOIN edges d ON d.to_id = m.id AND d.type = 'DEFINED_IN'
LEFT JOIN elements child ON child.id = d.from_id AND child.kind != 'module'
WHERE m.kind = 'module'
GROUP BY m.id
ORDER BY elements DESC, loc DESC, module
LIMIT %d`, limit))
	if err != nil {
		return nil, fmt.Errorf("query overview top modules: %w", err)
	}
	return rows, nil
}

// queryOverviewEntryPoints returns likely starting points for targeted follow-up.
func queryOverviewEntryPoints(store *graph.SQLiteStore, limit int) ([]map[string]any, error) {
	rows, err := store.RunSQL(fmt.Sprintf(`SELECT
	e.language AS language,
	e.kind AS kind,
	e.fq_name AS fq_name,
	e.path AS path,
	e.start_line AS line,
	(SELECT COUNT(*) FROM edges c WHERE c.type = 'CALLS' AND c.from_id = e.id) AS outgoing_calls,
	(SELECT COUNT(*) FROM edges c WHERE c.type = 'CALLS' AND c.to_id = e.id) AS incoming_calls
FROM elements e
WHERE e.kind IN ('function', 'method')
	AND (
		e.name = 'main'
		OR e.name = 'Execute'
		OR e.name = 'Run'
		OR e.name LIKE 'run%%'
		OR e.fq_name LIKE '%%.__main__%%'
	)
ORDER BY
	CASE
		WHEN e.name = 'main' THEN 0
		WHEN e.name = 'Execute' THEN 1
		WHEN e.name = 'Run' THEN 2
		ELSE 3
	END,
	outgoing_calls DESC,
	fq_name
LIMIT %d`, limit))
	if err != nil {
		return nil, fmt.Errorf("query overview entry points: %w", err)
	}
	return rows, nil
}

// queryOverviewDependencySketch returns local module-to-module call and import edges.
func queryOverviewDependencySketch(store *graph.SQLiteStore, limit int) ([]map[string]any, error) {
	rows, err := store.RunSQL(fmt.Sprintf(`WITH call_deps AS (
	SELECT
		sm.fq_name AS source_module,
		tm.fq_name AS target_module,
		'CALLS' AS edge_type,
		COUNT(*) AS edges
	FROM edges c
	JOIN elements sf ON sf.id = c.from_id
	JOIN elements tf ON tf.id = c.to_id
	JOIN edges sd ON sd.from_id = sf.id AND sd.type = 'DEFINED_IN'
	JOIN edges td ON td.from_id = tf.id AND td.type = 'DEFINED_IN'
	JOIN elements sm ON sm.id = sd.to_id AND sm.kind = 'module'
	JOIN elements tm ON tm.id = td.to_id AND tm.kind = 'module'
	WHERE c.type = 'CALLS' AND sm.id != tm.id
	GROUP BY sm.id, tm.id
),
import_deps AS (
	SELECT
		sm.fq_name AS source_module,
		COALESCE(target_module.fq_name, target.fq_name) AS target_module,
		'IMPORTS' AS edge_type,
		COUNT(*) AS edges
	FROM edges i
	JOIN elements sm ON sm.id = i.from_id AND sm.kind = 'module'
	JOIN elements target ON target.id = i.to_id
	LEFT JOIN edges td ON td.from_id = target.id AND td.type = 'DEFINED_IN'
	LEFT JOIN elements target_module ON target_module.id = td.to_id AND target_module.kind = 'module'
	WHERE i.type = 'IMPORTS'
	GROUP BY sm.id, target_module.id, target.id
)
SELECT source_module, target_module, edge_type, edges
FROM (
	SELECT * FROM call_deps
	UNION ALL
	SELECT * FROM import_deps
)
ORDER BY edges DESC, source_module, target_module
LIMIT %d`, limit))
	if err != nil {
		return nil, fmt.Errorf("query overview dependency sketch: %w", err)
	}
	return rows, nil
}

// queryOverviewExportSurface returns language-aware module export counts.
func queryOverviewExportSurface(store *graph.SQLiteStore, limit int) ([]map[string]any, error) {
	rows, err := store.RunSQL(fmt.Sprintf(`SELECT * FROM (
SELECT
	'python' AS language,
	src.fq_name AS module,
	src.path AS path,
	COUNT(*) AS exports
FROM edges e
JOIN elements src ON src.id = e.from_id
WHERE e.type = 'EXPORTS'
GROUP BY src.id
UNION ALL
SELECT
	m.language AS language,
	m.fq_name AS module,
	m.path AS path,
	COUNT(*) AS exports
FROM edges d
JOIN elements e ON e.id = d.from_id
JOIN elements m ON m.id = d.to_id
WHERE d.type = 'DEFINED_IN'
	AND m.kind = 'module'
	AND m.language IN ('go', 'rust')
	AND e.kind != 'module'
	AND e.visibility = 'public'
GROUP BY m.id
) ORDER BY exports DESC, language, module
LIMIT %d`, limit))
	if err != nil {
		return nil, fmt.Errorf("query overview export surface: %w", err)
	}
	return rows, nil
}

// queryOverviewModuleCalls ranks modules by incoming or outgoing call volume.
func queryOverviewModuleCalls(store *graph.SQLiteStore, edgeEnd string, limit int) ([]map[string]any, error) {
	if edgeEnd != "from_id" && edgeEnd != "to_id" {
		return nil, fmt.Errorf("unsupported module call edge end %q", edgeEnd)
	}
	rows, err := store.RunSQL(fmt.Sprintf(`SELECT
	m.language AS language,
	m.fq_name AS module,
	COUNT(*) AS calls
FROM edges c
JOIN elements f ON f.id = c.%s
JOIN edges d ON d.from_id = f.id AND d.type = 'DEFINED_IN'
JOIN elements m ON m.id = d.to_id AND m.kind = 'module'
WHERE c.type = 'CALLS'
GROUP BY m.id
ORDER BY calls DESC, module
LIMIT %d`, edgeEnd, limit))
	if err != nil {
		return nil, fmt.Errorf("query overview module calls: %w", err)
	}
	return rows, nil
}

// queryOverviewKeyElements returns high-signal elements ranked by graph activity.
func queryOverviewKeyElements(store *graph.SQLiteStore, limit int) ([]map[string]any, error) {
	rows, err := store.RunSQL(fmt.Sprintf(`SELECT
	e.language AS language,
	e.kind AS kind,
	e.fq_name AS fq_name,
	m.fq_name AS module,
	e.path AS path,
	e.start_line AS line,
	e.loc AS loc,
	(SELECT COUNT(*) FROM edges c WHERE c.type = 'CALLS' AND c.from_id = e.id) AS outgoing_calls,
	(SELECT COUNT(*) FROM edges c WHERE c.type = 'CALLS' AND c.to_id = e.id) AS incoming_calls
FROM elements e
LEFT JOIN edges d ON d.from_id = e.id AND d.type = 'DEFINED_IN'
LEFT JOIN elements m ON m.id = d.to_id AND m.kind = 'module'
WHERE e.kind IN ('function', 'method', 'class', 'struct', 'interface', 'trait', 'enum', 'impl')
ORDER BY (outgoing_calls + incoming_calls) DESC, loc DESC, fq_name
LIMIT %d`, limit))
	if err != nil {
		return nil, fmt.Errorf("query overview key elements: %w", err)
	}
	return rows, nil
}

// queryOverviewEntryRelationships returns compact callees and callers for entry points.
func queryOverviewEntryRelationships(store *graph.SQLiteStore, limit int) ([]map[string]any, error) {
	rows, err := store.RunSQL(fmt.Sprintf(`WITH entries AS (
	SELECT
		e.id,
		e.fq_name,
		(SELECT COUNT(*) FROM edges c WHERE c.type = 'CALLS' AND c.from_id = e.id) AS outgoing_calls
	FROM elements e
	WHERE e.kind IN ('function', 'method')
		AND (
			e.name = 'main'
			OR e.name = 'Execute'
			OR e.name = 'Run'
			OR e.name LIKE 'run%%'
			OR e.fq_name LIKE '%%.__main__%%'
		)
	ORDER BY
		CASE
			WHEN e.name = 'main' THEN 0
			WHEN e.name = 'Execute' THEN 1
			WHEN e.name = 'Run' THEN 2
			ELSE 3
		END,
		outgoing_calls DESC,
		e.fq_name
	LIMIT %d
)
SELECT
	'callee' AS direction,
	entries.fq_name AS entry_point,
	target.fq_name AS related_fq_name,
	target.kind AS related_kind
FROM entries
JOIN edges c ON c.from_id = entries.id AND c.type = 'CALLS'
JOIN elements target ON target.id = c.to_id
UNION ALL
SELECT
	'caller' AS direction,
	entries.fq_name AS entry_point,
	source.fq_name AS related_fq_name,
	source.kind AS related_kind
FROM entries
JOIN edges c ON c.to_id = entries.id AND c.type = 'CALLS'
JOIN elements source ON source.id = c.from_id
ORDER BY entry_point, direction, related_fq_name
LIMIT %d`, limit, limit))
	if err != nil {
		return nil, fmt.Errorf("query overview entry relationships: %w", err)
	}
	return rows, nil
}

// suggestOverviewCommands returns deterministic follow-up commands from overview rows.
func suggestOverviewCommands(modules []map[string]any, entries []map[string]any) []string {
	commands := []string{
		"treelines module-graph",
		"treelines list . --kind module",
	}
	if len(modules) > 0 {
		if module, ok := modules[0]["module"].(string); ok && module != "" {
			commands = append(commands, fmt.Sprintf("treelines module-graph %q", module))
			commands = append(commands, fmt.Sprintf("treelines list %q", module))
		}
	}
	if len(entries) > 0 {
		if fqName, ok := entries[0]["fq_name"].(string); ok && fqName != "" {
			commands = append(commands, fmt.Sprintf("treelines callees %q", fqName))
			commands = append(commands, fmt.Sprintf("treelines uses %q", fqName))
		}
	}
	return commands
}

// normalizeOverviewDepth validates and normalizes the overview depth flag.
func normalizeOverviewDepth(depth int) (int, error) {
	if depth < 1 || depth > 3 {
		return 0, fmt.Errorf("--depth must be 1, 2, or 3")
	}
	return depth, nil
}

// normalizePositiveLimit returns fallback when a limit is zero or negative.
func normalizePositiveLimit(limit int, fallback int) int {
	if limit <= 0 {
		return fallback
	}
	return limit
}

// printOverview emits the compact text overview used by humans and agents.
func printOverview(data *overviewData) error {
	_, _ = fmt.Fprintf(stdoutWriter(), "Treelines overview (depth %d, limit %d)\n", data.Depth, data.Limit)
	_, _ = fmt.Fprintln(stdoutWriter())
	_, _ = fmt.Fprintln(stdoutWriter(), "Project")
	_, _ = fmt.Fprintf(stdoutWriter(), "  elements: %v\n", data.Project["total_elements"])
	_, _ = fmt.Fprintf(stdoutWriter(), "  edges:    %v\n", data.Project["total_edges"])
	_, _ = fmt.Fprintf(stdoutWriter(), "  files:    %v\n", data.Project["source_files"])

	printOverviewSection("Languages", data.ElementsByLanguage)
	printOverviewSection("Element Kinds", data.ElementsByKind)
	printOverviewSection("Edge Types", data.EdgesByType)
	printOverviewSection("Top Modules", data.TopModules)
	printOverviewSection("Entry Points", data.EntryPoints)

	if data.Depth >= 2 {
		printOverviewSection("Dependency Sketch", data.DependencySketch)
		printOverviewSection("Export Surface", data.ExportSurface)
		printOverviewSection("Top Outgoing Modules", data.OutgoingModules)
		printOverviewSection("Top Incoming Modules", data.IncomingModules)
		_, _ = fmt.Fprintln(stdoutWriter())
		_, _ = fmt.Fprintln(stdoutWriter(), "Suggested Next Commands")
		for _, command := range data.SuggestedCommands {
			_, _ = fmt.Fprintf(stdoutWriter(), "  %s\n", command)
		}
	}

	if data.Depth >= 3 {
		printOverviewSection("Key Elements", data.KeyElements)
		printOverviewSection("Entry Relationships", data.EntryRelationships)
	}
	return nil
}

// printOverviewSection prints a titled table or a stable empty marker.
func printOverviewSection(title string, rows []map[string]any) {
	_, _ = fmt.Fprintln(stdoutWriter())
	_, _ = fmt.Fprintln(stdoutWriter(), title)
	if len(rows) == 0 {
		_, _ = fmt.Fprintln(stdoutWriter(), "  none")
		return
	}
	_ = printTable(rows)
}
