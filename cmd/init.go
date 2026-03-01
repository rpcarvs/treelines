package cmd

import (
	"fmt"
	"os"
	"path/filepath"

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
	defer store.Close()

	if err := store.CreateSchema(); err != nil {
		return fmt.Errorf("create schema: %w", err)
	}

	logInfo("Initialized treelines in %s", tlDir)
	return nil
}
