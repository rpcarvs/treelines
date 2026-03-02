package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

var queryFile string

var queryCmd = &cobra.Command{
	Use:   "query [sql]",
	Short: "Run a raw SQL query against the database",
	Long: `Run a raw SQL query against the database. Use this for ad-hoc
analysis that the structured commands (element, search, list, uses,
callees) don't cover.

SQL can be provided as a positional argument or read from a file via --file.
Use --file - to read from stdin, which avoids shell quoting issues.

The database has two tables:
  elements - id, language, kind, name, fq_name, path, start_line, end_line,
             loc, signature, visibility, docstring, body
  edges    - from_id, to_id, type

Edge types: CALLS, CONTAINS, DEFINED_IN, IMPLEMENTS, EXTENDS

Examples:
  echo "SELECT fq_name, loc FROM elements WHERE kind = 'function' ORDER BY loc DESC LIMIT 10" | lines query --file -
  lines query --file my_query.sql`,
	Args: cobra.MaximumNArgs(1),
	RunE: runQuery,
}

func init() {
	queryCmd.Flags().StringVar(&queryFile, "file", "", "Read SQL from file (use '-' for stdin)")
	rootCmd.AddCommand(queryCmd)
}

// runQuery executes a raw SQL query against the database.
func runQuery(cmd *cobra.Command, args []string) error {
	sql, err := resolveQuerySQL(args)
	if err != nil {
		return err
	}

	root, err := resolveRoot()
	if err != nil {
		return err
	}

	store, err := openStore(root)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	results, err := store.RunSQL(sql)
	if err != nil {
		return fmt.Errorf("run query: %w", err)
	}

	if len(results) == 0 {
		logInfo("Query returned no results")
		return nil
	}

	return output(results)
}

// resolveQuerySQL reads SQL from args, a file, or stdin.
func resolveQuerySQL(args []string) (string, error) {
	if queryFile != "" {
		var data []byte
		var err error
		if queryFile == "-" {
			data, err = io.ReadAll(os.Stdin)
		} else {
			data, err = os.ReadFile(queryFile)
		}
		if err != nil {
			return "", fmt.Errorf("read sql: %w", err)
		}
		return string(data), nil
	}
	if len(args) == 0 {
		return "", fmt.Errorf("provide SQL as an argument or use --file")
	}
	return args[0], nil
}
