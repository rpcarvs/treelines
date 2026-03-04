package skillinstaller

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	contextBlockBegin = "<!-- LINES-CODEBASE-EXPLORATION:BEGIN -->"
	contextBlockEnd   = "<!-- LINES-CODEBASE-EXPLORATION:END -->"
)

var (
	managedContextBlockPattern = regexp.MustCompile(`(?s)` + regexp.QuoteMeta(contextBlockBegin) + `.*?` + regexp.QuoteMeta(contextBlockEnd))
	legacyHeaderPattern        = regexp.MustCompile(`(?m)^# MANDATORY codebase exploration.*$`)
	topHeadingPattern          = regexp.MustCompile(`(?m)^# `)
)

const mandatoryContextBody = `# MANDATORY codebase exploration for Python, Rust and Go

Always invoke the lines-codebase-exploration skill before exploring these languages codebases. Do not use other exploration paths as a substitute.

Red flag: If you're about to Explore or use Read/Glob/Grep to understand the referred languages codebase, STOP. You must invoke the lines-codebase-exploration skill first. Exploring without lines is never the faster path.

Use full file reads if necessary.`

// InstallCodexContext creates or updates the global Codex context policy block.
func InstallCodexContext() (string, error) {
	path, err := codexContextPath()
	if err != nil {
		return "", err
	}
	if err := InstallContextAtPath(path); err != nil {
		return "", err
	}
	return path, nil
}

// InstallClaudeContext creates or updates the global Claude context policy block.
func InstallClaudeContext() (string, error) {
	path, err := claudeContextPath()
	if err != nil {
		return "", err
	}
	if err := InstallContextAtPath(path); err != nil {
		return "", err
	}
	return path, nil
}

// InstallContextAtPath creates or updates the managed policy block at a given path.
func InstallContextAtPath(path string) error {
	return installManagedContext(path)
}

// codexContextPath resolves the global Codex AGENTS.md path.
func codexContextPath() (string, error) {
	codexHome := os.Getenv("CODEX_HOME")
	if codexHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		codexHome = filepath.Join(home, ".codex")
	}
	return filepath.Join(codexHome, "AGENTS.md"), nil
}

// claudeContextPath resolves the global Claude CLAUDE.md path.
func claudeContextPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".claude", "CLAUDE.md"), nil
}

// installManagedContext writes the managed policy block into a target context file.
func installManagedContext(path string) error {
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("read context file %s: %w", path, err)
	}

	updated := upsertContextBlock(string(existing))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return fmt.Errorf("create context directory %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write context file %s: %w", path, err)
	}
	return nil
}

// upsertContextBlock replaces managed/legacy policy blocks and appends the latest block.
func upsertContextBlock(content string) string {
	managedBlock := contextBlockBegin + "\n" + mandatoryContextBody + "\n" + contextBlockEnd

	if managedContextBlockPattern.MatchString(content) {
		return strings.TrimSpace(managedContextBlockPattern.ReplaceAllString(content, managedBlock)) + "\n"
	}

	legacy := removeLegacyContextBlock(content)
	legacy = strings.TrimRight(legacy, "\n\t ")
	if legacy == "" {
		return managedBlock + "\n"
	}
	return legacy + "\n\n" + managedBlock + "\n"
}

// removeLegacyContextBlock removes previous unmanaged mandatory block variants.
func removeLegacyContextBlock(content string) string {
	loc := legacyHeaderPattern.FindStringIndex(content)
	if loc == nil {
		return content
	}

	start := loc[0]
	rest := content[start:]

	// Preferred legacy end marker sentence.
	if end := strings.Index(rest, "Use full file reads if necessary."); end >= 0 {
		endAbs := start + end + len("Use full file reads if necessary.")
		endAbs = consumeFollowingNewline(content, endAbs)
		return content[:start] + content[endAbs:]
	}
	if end := strings.Index(rest, "Use full file reads if necessary"); end >= 0 {
		endAbs := start + end + len("Use full file reads if necessary")
		endAbs = consumeLineRemainder(content, endAbs)
		endAbs = consumeFollowingNewline(content, endAbs)
		return content[:start] + content[endAbs:]
	}

	// Fallback: remove until next top-level heading or EOF.
	firstLineEnd := strings.Index(rest, "\n")
	if firstLineEnd < 0 {
		return content[:start]
	}
	searchStart := firstLineEnd + 1
	nextHeadingRel := topHeadingPattern.FindStringIndex(rest[searchStart:])
	if nextHeadingRel != nil {
		endAbs := start + searchStart + nextHeadingRel[0]
		return content[:start] + content[endAbs:]
	}

	return content[:start]
}

func consumeLineRemainder(content string, idx int) int {
	for idx < len(content) && content[idx] != '\n' {
		idx++
	}
	return idx
}

func consumeFollowingNewline(content string, idx int) int {
	for idx < len(content) && (content[idx] == '\n' || content[idx] == '\r') {
		idx++
	}
	return idx
}
