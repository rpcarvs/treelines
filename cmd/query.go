package cmd

import (
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
)

var queryFile string
var querySchema bool

var queryCmd = &cobra.Command{
	Use:   "query [sql]",
	Short: "Run a raw SQL query against the database",
	Long: `Run a raw SQL query against the database. Use this for ad-hoc
analysis that the structured commands (element, search, list, uses,
callees, imports, module-graph) don't cover.

SQL can be provided as a positional argument or read from a file via --file.
Use --file - to read from stdin, which avoids shell quoting issues.
Use --schema for a quick schema and sample-query reference.

The database has two tables:
  elements - id, language, kind, name, fq_name, path, start_line, end_line,
             loc, signature, visibility, docstring, body
  edges    - from_id, to_id, type

Edge types: CALLS, IMPORTS, EXPORTS, CONTAINS, DEFINED_IN, IMPLEMENTS, EXTENDS

Examples:
  echo "SELECT fq_name, loc FROM elements WHERE kind = 'function' ORDER BY loc DESC LIMIT 10" | lines query --file -
  lines query --file my_query.sql`,
	Args: cobra.MaximumNArgs(1),
	RunE: runQuery,
}

func init() {
	queryCmd.Flags().StringVar(&queryFile, "file", "", "Read SQL from file (use '-' for stdin)")
	queryCmd.Flags().BoolVar(&querySchema, "schema", false, "Show schema and sample queries")
	rootCmd.AddCommand(queryCmd)
}

// runQuery executes a raw SQL query against the database.
func runQuery(cmd *cobra.Command, args []string) error {
	if querySchema {
		if queryFile != "" || len(args) > 0 {
			return fmt.Errorf("--schema cannot be used with SQL input")
		}
		return outputQuerySchema()
	}

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

// outputQuerySchema prints SQL schema reference and sample queries.
func outputQuerySchema() error {
	if flagJSON {
		return outputJSON(map[string]any{
			"tables": []map[string]any{
				{
					"name": "elements",
					"columns": []string{
						"id", "language", "kind", "name", "fq_name", "path",
						"start_line", "end_line", "loc", "signature", "visibility", "docstring", "body",
					},
				},
				{
					"name":    "edges",
					"columns": []string{"from_id", "to_id", "type"},
				},
			},
			"edge_types": []string{"CALLS", "IMPORTS", "EXPORTS", "CONTAINS", "DEFINED_IN", "IMPLEMENTS", "EXTENDS"},
			"samples": []string{
				"SELECT kind, COUNT(*) AS c FROM elements GROUP BY kind ORDER BY c DESC",
				"SELECT language, kind, fq_name, path FROM elements WHERE kind='module' ORDER BY fq_name",
				"SELECT type, COUNT(*) AS c FROM edges GROUP BY type ORDER BY c DESC",
			},
		})
	}

	_, _ = fmt.Fprintln(os.Stdout, "Tables:")
	_, _ = fmt.Fprintln(os.Stdout, "  elements(id, language, kind, name, fq_name, path, start_line, end_line, loc, signature, visibility, docstring, body)")
	_, _ = fmt.Fprintln(os.Stdout, "  edges(from_id, to_id, type)")
	_, _ = fmt.Fprintln(os.Stdout, "Edge types:")
	_, _ = fmt.Fprintln(os.Stdout, "  CALLS, IMPORTS, EXPORTS, CONTAINS, DEFINED_IN, IMPLEMENTS, EXTENDS")
	_, _ = fmt.Fprintln(os.Stdout, "Sample queries:")
	_, _ = fmt.Fprintln(os.Stdout, "  SELECT kind, COUNT(*) AS c FROM elements GROUP BY kind ORDER BY c DESC")
	_, _ = fmt.Fprintln(os.Stdout, "  SELECT language, kind, fq_name, path FROM elements WHERE kind='module' ORDER BY fq_name")
	_, _ = fmt.Fprintln(os.Stdout, "  SELECT type, COUNT(*) AS c FROM edges GROUP BY type ORDER BY c DESC")
	return nil
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
