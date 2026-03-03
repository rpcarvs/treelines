package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"lines/internal/extractor"
	"lines/internal/model"
	"lines/internal/parser"
	"lines/internal/scanner"
	"lines/internal/watcher"

	"github.com/spf13/cobra"
)

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Watch for file changes and re-index automatically",
	RunE:  runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

// runServe watches for file changes and re-indexes automatically.
func runServe(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot()
	if err != nil {
		return err
	}
	releaseWriterLock, err := acquireWriterLock(root, "serve")
	if err != nil {
		return fmt.Errorf("acquire writer lock: %w", err)
	}
	defer releaseWriterLock()

	store, err := openStore(root)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = store.Close() }()

	w, err := watcher.New(root)
	if err != nil {
		return fmt.Errorf("create watcher: %w", err)
	}
	defer func() { _ = w.Close() }()

	logInfo("Watching for changes...")

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	p := parser.NewParser()
	defer p.Close()

	for {
		select {
		case batch, ok := <-w.Events():
			if !ok {
				return nil
			}
			var reindexed bool
			for _, path := range batch {
				relPath, _ := filepath.Rel(root, path)
				lang := scanner.LangForExt(filepath.Ext(path))
				if lang == "" {
					continue
				}

				if err := store.DeleteEdgesForFile(relPath); err != nil {
					logVerbose("Delete edges error %s: %v", relPath, err)
				}
				if err := store.DeleteElementsByFile(relPath); err != nil {
					logVerbose("Delete elements error %s: %v", relPath, err)
				}

				result, err := p.ParseFile(path, relPath, lang)
				if err != nil {
					logVerbose("Parse error %s: %v", relPath, err)
					continue
				}
				ext := extractor.ForLanguage(lang)
				if ext == nil {
					result.Tree.Close()
					continue
				}

				extracted, err := ext.Extract(result)
				result.Tree.Close()
				if err != nil {
					logVerbose("Extract error %s: %v", relPath, err)
					continue
				}

				for _, el := range extracted.Elements {
					if err := store.UpsertElement(el); err != nil {
						logVerbose("Upsert element error: %v", err)
					}
				}
				for _, edge := range extracted.Edges {
					if err := store.UpsertEdge(edge); err != nil {
						logVerbose("Upsert edge error: %v", err)
					}
				}

				reindexed = true
				logInfo("Re-indexed %s", relPath)
			}
			if reindexed {
				logInfo("Resolving cross-file edges...")
				allElements, err := store.GetAllElements()
				if err != nil {
					logVerbose("Get all elements for cross-ref: %v", err)
					continue
				}
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

		case <-sigCh:
			logInfo("Shutting down...")
			return nil
		}
	}
}
