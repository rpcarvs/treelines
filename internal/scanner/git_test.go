package scanner

import (
	"os/exec"
	"testing"
)

func TestIsGitRepoUsesGitWorkTreeDetection(t *testing.T) {
	root := t.TempDir()
	if IsGitRepo(root) {
		t.Fatalf("temp dir without git repo should not be detected as git repo")
	}

	cmd := exec.Command("git", "init")
	cmd.Dir = root
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git init failed: %v\n%s", err, output)
	}

	if !IsGitRepo(root) {
		t.Fatalf("git repository should be detected")
	}
}
