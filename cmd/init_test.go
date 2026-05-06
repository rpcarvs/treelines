package cmd

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// TestInitFromSubdirInitializesGitRoot verifies init never creates nested stores.
func TestInitFromSubdirInitializesGitRoot(t *testing.T) {
	root := initGitRepo(t)
	subdir := filepath.Join(root, "internal", "example")
	if err := os.MkdirAll(subdir, 0o755); err != nil {
		t.Fatalf("create subdir: %v", err)
	}

	restore := chdir(t, subdir)
	defer restore()

	if err := runInit(initCmd, nil); err != nil {
		t.Fatalf("run init: %v", err)
	}

	rootDB := filepath.Join(root, ".treelines", "codestore.db")
	nestedDB := filepath.Join(subdir, ".treelines", "codestore.db")
	if _, err := os.Stat(rootDB); err != nil {
		t.Fatalf("expected root database: %v", err)
	}
	if _, err := os.Stat(nestedDB); !os.IsNotExist(err) {
		t.Fatalf("nested database exists or stat failed unexpectedly: %v", err)
	}
}

// TestResolveRootOutsideGitRepoReturnsHelpfulError verifies non-git usage fails clearly.
func TestResolveRootOutsideGitRepoReturnsHelpfulError(t *testing.T) {
	restore := chdir(t, t.TempDir())
	defer restore()

	_, err := resolveRoot()
	if err == nil {
		t.Fatalf("expected error outside git repository")
	}
	if !strings.Contains(err.Error(), "treelines requires a Git repository") {
		t.Fatalf("unexpected error: %v", err)
	}
}

// initGitRepo creates a temporary Git repository for command tests.
func initGitRepo(t *testing.T) string {
	t.Helper()

	root := t.TempDir()
	cmd := exec.Command("git", "init")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, output)
	}
	return root
}

// chdir changes the process working directory and returns a restore function.
func chdir(t *testing.T, dir string) func() {
	t.Helper()

	previous, err := os.Getwd()
	if err != nil {
		t.Fatalf("get working directory: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("change directory: %v", err)
	}
	return func() {
		if err := os.Chdir(previous); err != nil {
			t.Fatalf("restore working directory: %v", err)
		}
	}
}
