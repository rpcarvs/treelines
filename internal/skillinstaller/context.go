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
	contextBlockBegin = "<!-- TREELINES-CODEBASE-EXPLORATION:BEGIN -->"
	contextBlockEnd   = "<!-- TREELINES-CODEBASE-EXPLORATION:END -->"
	legacyBlockBegin  = "<!-- LINES-CODEBASE-EXPLORATION:BEGIN -->"
	legacyBlockEnd    = "<!-- LINES-CODEBASE-EXPLORATION:END -->"
	pointerBlockBegin = "<!-- TREELINES-CLAUDE-POINTER:BEGIN -->"
	pointerBlockEnd   = "<!-- TREELINES-CLAUDE-POINTER:END -->"
)

var (
	managedContextBlockPattern = regexp.MustCompile(`(?s)` + regexp.QuoteMeta(contextBlockBegin) + `.*?` + regexp.QuoteMeta(contextBlockEnd))
	legacyManagedBlockPattern  = regexp.MustCompile(`(?s)` + regexp.QuoteMeta(legacyBlockBegin) + `.*?` + regexp.QuoteMeta(legacyBlockEnd))
	managedPointerBlockPattern = regexp.MustCompile(`(?s)` + regexp.QuoteMeta(pointerBlockBegin) + `.*?` + regexp.QuoteMeta(pointerBlockEnd))
	legacyHeaderPattern        = regexp.MustCompile(`(?m)^# MANDATORY codebase exploration.*$`)
	topHeadingPattern          = regexp.MustCompile(`(?m)^# `)
)

const mandatoryContextBody = `# MANDATORY codebase exploration for Python, Rust and Go

Always invoke the ` + "`treelines-codebase-exploration`" + ` skill before exploring these languages codebases. Do not use other exploration paths as a substitute.

Red flag: If you're about to Explore or use Read/Glob/Grep to understand the referred languages codebase, STOP. You must invoke the treelines-codebase-exploration skill first. Exploring without treelines is never the faster path.

Use full file reads if necessary.`

const claudeLocalPointer = "See [AGENTS.md](./AGENTS.md)\n"
const claudeLocalPointerBody = "See [AGENTS.md](./AGENTS.md)"

// CodexContextPath resolves the global Codex AGENTS.md path.
func CodexContextPath() (string, error) {
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

// ClaudeContextPath resolves the global Claude CLAUDE.md path.
func ClaudeContextPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".claude", "CLAUDE.md"), nil
}

// InstallCodexContext creates or updates the global Codex context policy block.
func InstallCodexContext() (string, string, error) {
	path, err := CodexContextPath()
	if err != nil {
		return "", "", err
	}
	action, err := InstallContextAtPath(path)
	if err != nil {
		return "", "", err
	}
	return path, action, nil
}

// InstallClaudeContext creates or updates the global Claude context policy block.
func InstallClaudeContext() (string, string, error) {
	path, err := ClaudeContextPath()
	if err != nil {
		return "", "", err
	}
	action, err := InstallContextAtPath(path)
	if err != nil {
		return "", "", err
	}
	return path, action, nil
}

// InstallContextAtPath creates or updates the managed policy block at a given path.
func InstallContextAtPath(path string) (string, error) {
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read context file %s: %w", path, err)
	}

	updated, action := upsertContextBlock(string(existing))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create context directory %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return "", fmt.Errorf("write context file %s: %w", path, err)
	}
	return action, nil
}

// InstallClaudePointerAtPath writes the local Claude pointer to AGENTS.md.
func InstallClaudePointerAtPath(path string) (string, error) {
	existing, err := os.ReadFile(path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return "", fmt.Errorf("read Claude pointer file %s: %w", path, err)
	}

	updated, action := upsertClaudePointerBlock(string(existing))
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", fmt.Errorf("create Claude pointer directory %s: %w", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return "", fmt.Errorf("write Claude pointer file %s: %w", path, err)
	}
	return action, nil
}

func upsertClaudePointerBlock(content string) (string, string) {
	managedBlock := pointerBlockBegin + "\n" + claudeLocalPointerBody + "\n" + pointerBlockEnd

	if managedPointerBlockPattern.MatchString(content) {
		replaced := strings.TrimSpace(managedPointerBlockPattern.ReplaceAllString(content, managedBlock)) + "\n"
		if replaced == content {
			return replaced, "unchanged"
		}
		return replaced, "updated"
	}

	legacy := strings.TrimSpace(content)
	if legacy == strings.TrimSpace(claudeLocalPointer) {
		return managedBlock + "\n", "updated"
	}

	trimmed := strings.TrimRight(content, "\n\t ")
	if trimmed == "" {
		return managedBlock + "\n", "appended"
	}
	return trimmed + "\n\n" + managedBlock + "\n", "appended"
}

// upsertContextBlock replaces managed/legacy policy blocks and appends the latest block.
func upsertContextBlock(content string) (string, string) {
	managedBlock := contextBlockBegin + "\n" + mandatoryContextBody + "\n" + contextBlockEnd

	if managedContextBlockPattern.MatchString(content) {
		replaced := strings.TrimSpace(managedContextBlockPattern.ReplaceAllString(content, managedBlock)) + "\n"
		if replaced == content {
			return replaced, "unchanged"
		}
		return replaced, "updated"
	}
	if legacyManagedBlockPattern.MatchString(content) {
		return strings.TrimSpace(legacyManagedBlockPattern.ReplaceAllString(content, managedBlock)) + "\n", "updated"
	}

	legacy := removeLegacyContextBlock(content)
	legacy = strings.TrimRight(legacy, "\n\t ")
	if legacy == "" {
		return managedBlock + "\n", "appended"
	}
	return legacy + "\n\n" + managedBlock + "\n", "appended"
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
