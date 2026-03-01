package cmd

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"lines/internal/graph"
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
	return filepath.Join(root, ".treelines", "db")
}

// openStore creates and opens a SQLiteStore at the resolved path.
func openStore(root string) (*graph.SQLiteStore, error) {
	store := graph.NewSQLiteStore()
	if err := store.Open(dbPath(root)); err != nil {
		return nil, err
	}
	return store, nil
}

// output writes data in the configured format.
func output(data any) error {
	if flagFormat == "json" {
		return outputJSON(data)
	}
	return outputText(data)
}

func outputJSON(data any) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(data)
}

func outputText(data any) error {
	switch v := data.(type) {
	case string:
		fmt.Println(v)
	case []any:
		for _, item := range v {
			fmt.Println(item)
		}
	default:
		b, err := json.MarshalIndent(v, "", "  ")
		if err != nil {
			return err
		}
		fmt.Println(string(b))
	}
	return nil
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
