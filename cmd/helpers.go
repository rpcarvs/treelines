package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"

	"lines/internal/graph"
	"lines/internal/model"
)

// resolveRoot returns the project root directory.
func resolveRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", fmt.Errorf("get working directory: %w", err)
	}
	return dir, nil
}

// dbPath returns the database path, using the flag or default.
func dbPath(root string) string {
	if flagDB != "" {
		return flagDB
	}
	return filepath.Join(root, ".treelines", "codestore.db")
}

// openStore creates and opens a SQLiteStore at the resolved path.
func openStore(root string) (*graph.SQLiteStore, error) {
	store := graph.NewSQLiteStore()
	if err := store.Open(dbPath(root)); err != nil {
		return nil, err
	}
	return store, nil
}

func output(data any) error {
	if flagNoBody {
		stripBodies(data)
	}
	if flagJSON {
		return outputJSON(data)
	}
	return outputCompact(data)
}

func stripBodies(data any) {
	switch v := data.(type) {
	case *model.Element:
		v.Body = ""
	case []model.Element:
		for i := range v {
			v[i].Body = ""
		}
	}
}

func outputJSON(data any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func outputCompact(data any) error {
	switch v := data.(type) {
	case *model.Element:
		return printElementDetail(v)
	case model.Element:
		return printElementDetail(&v)
	case []model.Element:
		return printElementList(v)
	case []map[string]any:
		return printTable(v)
	case map[string]any:
		return printTable([]map[string]any{v})
	default:
		return outputJSON(data)
	}
}

func printElementDetail(el *model.Element) error {
	fmt.Printf("%s %s %s (%s)\n", el.Language, el.Kind, el.FQName, el.Visibility)
	fmt.Printf("  %s:%d-%d (%d loc)\n", el.Path, el.StartLine, el.EndLine, el.LOC)
	if el.Signature != "" {
		fmt.Printf("  %s\n", el.Signature)
	}
	if el.Docstring != "" {
		for _, line := range strings.Split(el.Docstring, "\n") {
			fmt.Printf("  # %s\n", line)
		}
	}
	if el.Body != "" {
		fmt.Println()
		fmt.Println(el.Body)
	}
	return nil
}

func printElementList(elements []model.Element) error {
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "KIND\tFQNAME\tPATH\tVIS\tLOC")
	for _, el := range elements {
		fmt.Fprintf(w, "%s\t%s\t%s:%d\t%s\t%d\n",
			el.Kind, el.FQName, el.Path, el.StartLine, el.Visibility, el.LOC)
	}
	return w.Flush()
}

func printTable(rows []map[string]any) error {
	if len(rows) == 0 {
		return nil
	}
	var cols []string
	for k := range rows[0] {
		cols = append(cols, k)
	}
	sort.Strings(cols)
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, strings.Join(cols, "\t"))
	for _, row := range rows {
		vals := make([]string, len(cols))
		for i, col := range cols {
			vals[i] = fmt.Sprintf("%v", row[col])
		}
		fmt.Fprintln(w, strings.Join(vals, "\t"))
	}
	return w.Flush()
}

func logVerbose(format string, args ...any) {
	if flagVerbose && !flagQuiet {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}

func logInfo(format string, args ...any) {
	if !flagQuiet {
		fmt.Fprintf(os.Stderr, format+"\n", args...)
	}
}
