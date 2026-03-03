package cmd

import (
	"fmt"
	"strconv"

	"github.com/spf13/cobra"
)

var statsCmd = &cobra.Command{
	Use:   "stats",
	Short: "Show element and edge counts from the database",
	Args:  cobra.NoArgs,
	RunE:  runStats,
}

func init() {
	rootCmd.AddCommand(statsCmd)
}

// runStats prints element and edge count statistics.
func runStats(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot()
	if err != nil {
		return err
	}

	store, err := openStore(root)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

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
    SELECT 'total_elements' AS section, 'total' AS label, COUNT(*) AS count
    FROM elements
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
		return fmt.Errorf("query stats snapshot: %w", err)
	}

	var (
		byKind        []map[string]any
		byLang        []map[string]any
		byEdge        []map[string]any
		totalElements int64
	)
	for _, row := range rows {
		section, _ := row["section"].(string)
		label, _ := row["label"].(string)
		count := toInt64(row["count"])
		entry := map[string]any{
			"count": count,
		}
		switch section {
		case "kind":
			entry["kind"] = label
			byKind = append(byKind, entry)
		case "language":
			entry["language"] = label
			byLang = append(byLang, entry)
		case "edge":
			entry["type"] = label
			byEdge = append(byEdge, entry)
		case "total_elements":
			totalElements = count
		}
	}
	var byLangSum int64
	for _, row := range byLang {
		byLangSum += toInt64(row["count"])
	}

	if flagJSON {
		result := map[string]any{
			"total_elements":       totalElements,
			"language_sum":         byLangSum,
			"language_sum_matches": byLangSum == totalElements,
			"elements_by_kind":     byKind,
			"elements_by_language": byLang,
			"edges_by_type":        byEdge,
		}
		return outputJSON(result)
	}

	fmt.Println("Elements by kind:")
	for _, row := range byKind {
		fmt.Printf("  %4v  %v\n", row["count"], row["kind"])
	}
	fmt.Println("Elements by language:")
	for _, row := range byLang {
		fmt.Printf("  %4v  %v\n", row["count"], row["language"])
	}
	fmt.Printf("Total elements: %d (language sum: %d, match: %t)\n", totalElements, byLangSum, byLangSum == totalElements)
	fmt.Println("Edges by type:")
	for _, row := range byEdge {
		fmt.Printf("  %4v  %v\n", row["count"], row["type"])
	}
	return nil
}

// toInt64 converts database numeric values into int64.
func toInt64(v any) int64 {
	switch n := v.(type) {
	case int64:
		return n
	case int:
		return int64(n)
	case int32:
		return int64(n)
	case int16:
		return int64(n)
	case int8:
		return int64(n)
	case uint64:
		return int64(n)
	case uint:
		return int64(n)
	case uint32:
		return int64(n)
	case uint16:
		return int64(n)
	case uint8:
		return int64(n)
	case []byte:
		i, _ := strconv.ParseInt(string(n), 10, 64)
		return i
	case string:
		i, _ := strconv.ParseInt(n, 10, 64)
		return i
	default:
		return 0
	}
}
