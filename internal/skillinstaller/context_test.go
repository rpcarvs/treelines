package skillinstaller

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallCodexContextCreatesManagedBlock(t *testing.T) {
	tmp := t.TempDir()
	codexHome := filepath.Join(tmp, "codex-home")
	t.Setenv("CODEX_HOME", codexHome)
	t.Setenv("HOME", tmp)

	path, err := InstallCodexContext()
	if err != nil {
		t.Fatalf("install codex context: %v", err)
	}
	expected := filepath.Join(codexHome, "AGENTS.md")
	if path != expected {
		t.Fatalf("expected %s, got %s", expected, path)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read context: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, contextBlockBegin) || !strings.Contains(content, contextBlockEnd) {
		t.Fatalf("managed markers missing:\n%s", content)
	}
	if !strings.Contains(content, "# MANDATORY codebase exploration for Python, Rust and Go") {
		t.Fatalf("mandatory heading missing:\n%s", content)
	}
}

func TestInstallClaudeContextAppendsIntoExistingFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	path := filepath.Join(tmp, ".claude", "CLAUDE.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	if err := os.WriteFile(path, []byte("# Existing\n\nKeep this.\n"), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	installedPath, err := InstallClaudeContext()
	if err != nil {
		t.Fatalf("install claude context: %v", err)
	}
	if installedPath != path {
		t.Fatalf("expected %s, got %s", path, installedPath)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read context: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, "# Existing") {
		t.Fatalf("existing content removed unexpectedly:\n%s", content)
	}
	if strings.Count(content, contextBlockBegin) != 1 {
		t.Fatalf("expected exactly one managed block begin marker:\n%s", content)
	}
}

func TestInstallCodexContextReplacesManagedBlock(t *testing.T) {
	tmp := t.TempDir()
	codexHome := filepath.Join(tmp, "codex-home")
	t.Setenv("CODEX_HOME", codexHome)
	t.Setenv("HOME", tmp)

	path := filepath.Join(codexHome, "AGENTS.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	old := "# Header\n\n" +
		contextBlockBegin + "\nold body\n" + contextBlockEnd + "\n\nTail\n"
	if err := os.WriteFile(path, []byte(old), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if _, err := InstallCodexContext(); err != nil {
		t.Fatalf("install codex context: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read context: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "old body") {
		t.Fatalf("old block content not replaced:\n%s", content)
	}
	if strings.Count(content, contextBlockBegin) != 1 || strings.Count(content, contextBlockEnd) != 1 {
		t.Fatalf("expected exactly one managed block:\n%s", content)
	}
	if !strings.Contains(content, "Tail") {
		t.Fatalf("unexpected removal of surrounding content:\n%s", content)
	}
}

func TestInstallClaudeContextReplacesLegacyBlock(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	path := filepath.Join(tmp, ".claude", "CLAUDE.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	legacy := `# Existing

# MANDATORY codebase exploration for Python, Rust and Go

Old text.
Use full file reads if necessary.

# Footer
`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if _, err := InstallClaudeContext(); err != nil {
		t.Fatalf("install claude context: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read context: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "Old text.") {
		t.Fatalf("legacy block content still present:\n%s", content)
	}
	if !strings.Contains(content, "# Existing") || !strings.Contains(content, "# Footer") {
		t.Fatalf("surrounding content should be preserved:\n%s", content)
	}
	if strings.Count(content, contextBlockBegin) != 1 {
		t.Fatalf("expected managed block once:\n%s", content)
	}
}

func TestInstallClaudeContextReplacesLegacyBlockWithoutEndSentence(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	path := filepath.Join(tmp, ".claude", "CLAUDE.md")
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}
	legacy := `# Intro

# MANDATORY codebase exploration for Python, Rust and Go

User edited text and removed expected ending marker.

# Next Section
still here
`
	if err := os.WriteFile(path, []byte(legacy), 0o644); err != nil {
		t.Fatalf("seed file: %v", err)
	}

	if _, err := InstallClaudeContext(); err != nil {
		t.Fatalf("install claude context: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read context: %v", err)
	}
	content := string(data)
	if strings.Contains(content, "User edited text") {
		t.Fatalf("legacy block content still present:\n%s", content)
	}
	if !strings.Contains(content, "# Next Section") || !strings.Contains(content, "still here") {
		t.Fatalf("content after legacy block should stay:\n%s", content)
	}
	if strings.Count(content, contextBlockBegin) != 1 {
		t.Fatalf("expected managed block once:\n%s", content)
	}
}

func TestInstallContextAtPathCreatesTargetFile(t *testing.T) {
	tmp := t.TempDir()
	path := filepath.Join(tmp, "project", "AGENTS.md")

	if err := InstallContextAtPath(path); err != nil {
		t.Fatalf("install context at path: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read context: %v", err)
	}
	content := string(data)
	if !strings.Contains(content, contextBlockBegin) || !strings.Contains(content, contextBlockEnd) {
		t.Fatalf("managed markers missing:\n%s", content)
	}
}
