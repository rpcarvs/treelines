package cmd

import (
	"fmt"

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

	byKind, err := store.RunSQL("SELECT kind, COUNT(*) as count FROM elements GROUP BY kind ORDER BY count DESC")
	if err != nil {
		return fmt.Errorf("query elements by kind: %w", err)
	}

	byLang, err := store.RunSQL("SELECT language, COUNT(*) as count FROM elements GROUP BY language ORDER BY count DESC")
	if err != nil {
		return fmt.Errorf("query elements by language: %w", err)
	}

	byEdge, err := store.RunSQL("SELECT type, COUNT(*) as count FROM edges GROUP BY type ORDER BY count DESC")
	if err != nil {
		return fmt.Errorf("query edges by type: %w", err)
	}

	if flagJSON {
		result := map[string]any{
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
	fmt.Println("Edges by type:")
	for _, row := range byEdge {
		fmt.Printf("  %4v  %v\n", row["count"], row["type"])
	}
	return nil
}
