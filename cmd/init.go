package cmd

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"
)

var initCmd = &cobra.Command{
	Use:   "init",
	Short: "Initialize treelines in the current directory",
	RunE:  runInit,
}

func init() {
	rootCmd.AddCommand(initCmd)
}

// runInit creates the .treelines directory and initializes the database schema.
func runInit(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot()
	if err != nil {
		return err
	}

	tlDir := filepath.Join(root, ".treelines")
	if err := os.MkdirAll(tlDir, 0o755); err != nil {
		return fmt.Errorf("create .treelines directory: %w", err)
	}

	store, err := openStore(root)
	if err != nil {
		return fmt.Errorf("open store: %w", err)
	}
	defer func() { _ = store.Close() }()

	if err := store.CreateSchema(); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	ensureGitignoreEntry(root)

	logInfo("Initialized treelines in %s", tlDir)
	return nil
}

// ensureGitignoreEntry appends .treelines/ to .gitignore if the file exists
// and does not already contain the entry.
func ensureGitignoreEntry(root string) {
	gitignorePath := filepath.Join(root, ".gitignore")
	f, err := os.Open(gitignorePath)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) == ".treelines/" {
			return
		}
	}
	_ = f.Close()

	out, err := os.OpenFile(gitignorePath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return
	}
	defer func() { _ = out.Close() }()
	_, _ = out.WriteString(".treelines/\n")
}
