package cmd

import (
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"

	"lines/internal/extractor"
	"lines/internal/parser"
	"lines/internal/watcher"

	"github.com/spf13/cobra"
)

var extToLang = map[string]string{
	".py": "python",
	".go": "go",
	".rs": "rust",
}

var serveCmd = &cobra.Command{
	Use:   "serve",
	Short: "Watch for file changes and re-index automatically",
	RunE:  runServe,
}

func init() {
	rootCmd.AddCommand(serveCmd)
}

func runServe(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot()
	if err != nil {
		return err
	}

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
			for _, path := range batch {
				relPath, _ := filepath.Rel(root, path)
				lang := langFromPath(path)
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
				defer result.Tree.Close()

				ext := extractor.ForLanguage(lang)
				if ext == nil {
					continue
				}

				extracted, err := ext.Extract(result)
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

				logInfo("Re-indexed %s", relPath)
			}

		case <-sigCh:
			logInfo("Shutting down...")
			return nil
		}
	}
}

func langFromPath(path string) string {
	return extToLang[filepath.Ext(path)]
}
