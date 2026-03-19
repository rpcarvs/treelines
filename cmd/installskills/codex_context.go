package installskills

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/rpcarvs/treelines/internal/skillinstaller"
	"github.com/spf13/cobra"
)

// newCodexContextCommand installs the managed Codex global context block.
func newCodexContextCommand() *cobra.Command {
	var local bool

	cmd := &cobra.Command{
		Use:   "codex-context",
		Short: "Install the lines policy block into Codex global AGENTS.md",
		Long: `Install or refresh the managed lines policy block for Codex.

Default target: ~/.codex/AGENTS.md (or $CODEX_HOME/AGENTS.md).
Use --local to target ./AGENTS.md in the current directory.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var installedPath string
			if local {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("resolve current directory: %w", err)
				}
				installedPath = filepath.Join(cwd, "AGENTS.md")
				if err := skillinstaller.InstallContextAtPath(installedPath); err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Updated Codex local context:")
			} else {
				path, err := skillinstaller.InstallCodexContext()
				if err != nil {
					return err
				}
				installedPath = path
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Updated Codex global context:")
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", installedPath)
			return nil
		},
	}
	cmd.Flags().BoolVar(&local, "local", false, "Write to ./AGENTS.md instead of global Codex context")
	return cmd
}
