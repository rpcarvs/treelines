package cmd

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"lines/internal/extractor"
	"lines/internal/model"
	"lines/internal/parser"
	"lines/internal/scanner"

	"github.com/spf13/cobra"
)

var updateCmd = &cobra.Command{
	Use:   "update",
	Short: "Incrementally re-index files changed since last commit",
	RunE:  runUpdate,
}

func init() {
	rootCmd.AddCommand(updateCmd)
}

// runUpdate re-indexes only files changed since the last indexed commit.
func runUpdate(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot()
	if err != nil {
		return err
	}

	if !scanner.IsGitRepo(root) {
		return fmt.Errorf("not a git repository; use 'lines index' for a full index")
	}

	lastCommitPath := filepath.Join(root, ".treelines", "last_commit")
	data, err := os.ReadFile(lastCommitPath)
	if err != nil {
		return fmt.Errorf("no previous index found; run 'lines index' first")
	}
	lastCommit := strings.TrimSpace(string(data))

	changed, err := scanner.ChangedFiles(root, lastCommit)
	if err != nil {
		return fmt.Errorf("detect changed files: %w", err)
	}

	if len(changed) == 0 {
		logInfo("No changes detected")
		return nil
	}

	store, err := openStore(root)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = store.Close() }()

	p := parser.NewParser()
	defer p.Close()

	var reindexed int

	for _, relPath := range changed {
		fileExt := filepath.Ext(relPath)
		lang := scanner.LangForExt(fileExt)
		if lang == "" {
			continue
		}

		absPath := filepath.Join(root, relPath)

		if err := store.DeleteEdgesForFile(relPath); err != nil {
			logVerbose("Delete edges error for %s: %v", relPath, err)
		}
		if err := store.DeleteElementsByFile(relPath); err != nil {
			logVerbose("Delete elements error for %s: %v", relPath, err)
		}

		if _, err := os.Stat(absPath); os.IsNotExist(err) {
			logVerbose("File deleted: %s", relPath)
			reindexed++
			continue
		}

		result, err := p.ParseFile(absPath, relPath, lang)
		if err != nil {
			logVerbose("Skip %s: %v", relPath, err)
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

		reindexed++
	}

	logInfo("Resolving cross-package calls...")
	allElements, err := store.GetAllElements()
	if err != nil {
		logVerbose("Get all elements for cross-ref: %v", err)
	} else {
		if err := store.DeleteEdgesByType(model.EdgeCalls); err != nil {
			logVerbose("Delete old CALLS edges: %v", err)
		}
		crossEdges := extractor.ResolveCrossPackageCalls(allElements, p, root)
		for _, e := range crossEdges {
			if err := store.UpsertEdge(e); err != nil {
				logVerbose("Upsert cross-ref edge: %v", err)
			}
		}
		logInfo("Resolved %d cross-package call edges", len(crossEdges))
	}

	commit, err := scanner.CurrentCommit(root)
	if err == nil {
		_ = os.WriteFile(lastCommitPath, []byte(commit), 0o644)
	}

	logInfo("Re-indexed %d file(s)", reindexed)
	return nil
}
