package installskills

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallCodexCommandUsesProviderWorkflow(t *testing.T) {
	tmp := t.TempDir()
	codexHome := filepath.Join(tmp, "codex-home")
	t.Setenv("CODEX_HOME", codexHome)
	t.Setenv("HOME", tmp)

	output := runInstallCommand(t, func() (string, error) { return "", nil }, "codex")

	if !strings.Contains(output, "Installed Codex global integration") {
		t.Fatalf("unexpected output:\n%s", output)
	}
	assertPathExists(t, filepath.Join(codexHome, "skills", "treelines-codebase-exploration", "SKILL.md"))
	assertPathExists(t, filepath.Join(codexHome, "AGENTS.md"))
	assertPathExists(t, filepath.Join(codexHome, "hooks.json"))
}

func TestInstallClaudeLocalCommandUsesProjectRoot(t *testing.T) {
	tmp := t.TempDir()
	root := filepath.Join(tmp, "repo")
	t.Setenv("HOME", tmp)

	output := runInstallCommand(t, func() (string, error) { return root, nil }, "claude", "--local")

	if !strings.Contains(output, "Installed Claude local integration") {
		t.Fatalf("unexpected output:\n%s", output)
	}
	assertPathExists(t, filepath.Join(root, "AGENTS.md"))
	assertPathExists(t, filepath.Join(root, "CLAUDE.md"))
	assertPathExists(t, filepath.Join(root, ".claude", "settings.json"))
	assertPathExists(t, filepath.Join(root, ".claude", "skills", "treelines-codebase-exploration", "SKILL.md"))
}

// runInstallCommand executes the install command with test-local IO.
func runInstallCommand(t *testing.T, projectRoot ProjectRootFunc, args ...string) string {
	t.Helper()

	var output bytes.Buffer
	cmd := NewCommand(projectRoot)
	cmd.SetOut(&output)
	cmd.SetErr(&output)
	cmd.SetArgs(args)

	if err := cmd.Execute(); err != nil {
		t.Fatalf("execute install %v: %v\n%s", args, err, output.String())
	}
	return output.String()
}

// assertPathExists fails when a required install artifact is missing.
func assertPathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected path %s to exist: %v", path, err)
	}
}
