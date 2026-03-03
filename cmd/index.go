package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"lines/internal/extractor"
	"lines/internal/model"
	"lines/internal/parser"
	"lines/internal/scanner"

	"github.com/spf13/cobra"
)

var indexCmd = &cobra.Command{
	Use:   "index",
	Short: "Index the codebase into the graph database",
	RunE:  runIndex,
}

func init() {
	rootCmd.AddCommand(indexCmd)
}

// runIndex parses all source files and stores elements and edges in the database.
func runIndex(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot()
	if err != nil {
		return err
	}
	releaseWriterLock, err := acquireWriterLock(root, "index")
	if err != nil {
		return fmt.Errorf("acquire writer lock: %w", err)
	}
	defer releaseWriterLock()

	store, err := openStore(root)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = store.Close() }()

	sc := scanner.NewScanner(root)
	files, err := sc.ScanAll()
	if err != nil {
		return fmt.Errorf("scan files: %w", err)
	}

	if err := store.Reset(); err != nil {
		return fmt.Errorf("reset store for full index: %w", err)
	}

	p := parser.NewParser()
	defer p.Close()

	var attemptedElements, attemptedEdges int

	for _, fi := range files {
		logVerbose("Parsing %s", fi.RelPath)

		result, err := p.ParseFile(fi.Path, fi.RelPath, fi.Language)
		if err != nil {
			logVerbose("Skip %s: %v", fi.RelPath, err)
			continue
		}
		ext := extractor.ForLanguage(fi.Language)
		if ext == nil {
			result.Tree.Close()
			continue
		}

		extracted, err := ext.Extract(result)
		result.Tree.Close()
		if err != nil {
			logVerbose("Extract error %s: %v", fi.RelPath, err)
			continue
		}

		for _, el := range extracted.Elements {
			if err := store.UpsertElement(el); err != nil {
				logVerbose("Upsert element error: %v", err)
			}
			attemptedElements++
		}

		for _, edge := range extracted.Edges {
			if err := store.UpsertEdge(edge); err != nil {
				logVerbose("Upsert edge error: %v", err)
			}
			attemptedEdges++
		}
	}

	logInfo("Resolving cross-file edges...")
	allElements, err := store.GetAllElements()
	if err != nil {
		logVerbose("Get all elements for cross-ref: %v", err)
	} else {
		if err := store.DeleteEdgesByType(model.EdgeCalls); err != nil {
			logVerbose("Delete old CALLS edges: %v", err)
		}
		if err := store.DeleteEdgesByType(model.EdgeImports); err != nil {
			logVerbose("Delete old IMPORTS edges: %v", err)
		}
		if err := store.DeleteEdgesByType(model.EdgeExports); err != nil {
			logVerbose("Delete old EXPORTS edges: %v", err)
		}
		crossEdges := extractor.ResolveCrossPackageCalls(allElements, p, root)
		for _, e := range crossEdges {
			if err := store.UpsertEdge(e); err != nil {
				logVerbose("Upsert cross-ref edge: %v", err)
			}
		}
		if err := store.DeleteDanglingEdgesByType(model.EdgeExtends); err != nil {
			logVerbose("Delete dangling EXTENDS edges: %v", err)
		}
		logInfo("Resolved %d cross-file edges", len(crossEdges))
	}

	if scanner.IsGitRepo(root) {
		commit, err := scanner.CurrentCommit(root)
		if err == nil {
			lastCommitPath := filepath.Join(root, ".treelines", "last_commit")
			_ = os.WriteFile(lastCommitPath, []byte(commit), 0o644)
		}
	}

	elementRows, err := store.RunSQL("SELECT COUNT(*) AS count FROM elements")
	if err != nil || len(elementRows) == 0 {
		logInfo("Indexed %d element upserts, %d edge upserts from %d files", attemptedElements, attemptedEdges, len(files))
		return nil
	}
	edgeRows, err := store.RunSQL("SELECT COUNT(*) AS count FROM edges")
	if err != nil || len(edgeRows) == 0 {
		logInfo("Indexed %d element upserts, %d edge upserts from %d files", attemptedElements, attemptedEdges, len(files))
		return nil
	}
	persistedElements := toInt64(elementRows[0]["count"])
	persistedEdges := toInt64(edgeRows[0]["count"])
	logInfo("Indexed %d files (persisted: %d elements, %d edges)", len(files), persistedElements, persistedEdges)
	return nil
}
