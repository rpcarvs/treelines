package installskills

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"lines/internal/skillinstaller"
)

// newClaudeContextCommand installs the managed Claude global context block.
func newClaudeContextCommand() *cobra.Command {
	var local bool

	cmd := &cobra.Command{
		Use:   "claude-context",
		Short: "Install the lines policy block into Claude global CLAUDE.md",
		Long: `Install or refresh the managed lines policy block for Claude.

Default target: ~/.claude/CLAUDE.md.
Use --local to target ./CLAUDE.md in the current directory.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			var installedPath string
			if local {
				cwd, err := os.Getwd()
				if err != nil {
					return fmt.Errorf("resolve current directory: %w", err)
				}
				installedPath = filepath.Join(cwd, "CLAUDE.md")
				if err := skillinstaller.InstallContextAtPath(installedPath); err != nil {
					return err
				}
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Updated Claude local context:")
			} else {
				path, err := skillinstaller.InstallClaudeContext()
				if err != nil {
					return err
				}
				installedPath = path
				_, _ = fmt.Fprintln(cmd.OutOrStdout(), "Updated Claude global context:")
			}
			_, _ = fmt.Fprintf(cmd.OutOrStdout(), "  %s\n", installedPath)
			return nil
		},
	}
	cmd.Flags().BoolVar(&local, "local", false, "Write to ./CLAUDE.md instead of global Claude context")
	return cmd
}
