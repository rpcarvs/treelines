package scanner

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// IsGitRepo checks whether a .git directory exists at the given root.
func IsGitRepo(root string) bool {
	info, err := os.Stat(filepath.Join(root, ".git"))
	if err != nil {
		return false
	}
	return info.IsDir()
}

// CurrentCommit returns the HEAD commit hash for the repo at root.
func CurrentCommit(root string) (string, error) {
	cmd := exec.Command("git", "rev-parse", "HEAD")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(out)), nil
}

// ChangedFiles returns file paths changed between sinceCommit and HEAD.
func ChangedFiles(root, sinceCommit string) ([]string, error) {
	cmd := exec.Command("git", "diff", "--name-only", sinceCommit, "HEAD")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		return nil, err
	}

	raw := strings.TrimSpace(string(out))
	if raw == "" {
		return nil, nil
	}
	return strings.Split(raw, "\n"), nil
}
