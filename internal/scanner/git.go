package scanner

import (
	"os/exec"
	"strings"
)

// IsGitRepo checks whether root is inside a Git work tree.
func IsGitRepo(root string) bool {
	cmd := exec.Command("git", "rev-parse", "--is-inside-work-tree")
	cmd.Dir = root
	out, err := cmd.Output()
	return err == nil && strings.TrimSpace(string(out)) == "true"
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
