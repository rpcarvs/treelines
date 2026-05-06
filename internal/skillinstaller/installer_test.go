package skillinstaller

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestInstallCodexSkillCreatesOnlySkillFile(t *testing.T) {
	tmp := t.TempDir()
	codexHome := filepath.Join(tmp, "codex-home")
	t.Setenv("CODEX_HOME", codexHome)
	t.Setenv("HOME", tmp)

	installedPath, err := InstallCodexSkill(false)
	if err != nil {
		t.Fatalf("install codex skill: %v", err)
	}

	expectedPath := filepath.Join(codexHome, "skills", skillDirName)
	if installedPath != expectedPath {
		t.Fatalf("expected %s, got %s", expectedPath, installedPath)
	}

	skillPath := filepath.Join(installedPath, "SKILL.md")

	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("missing SKILL.md: %v", err)
	}
	assertInstalledSharedSkill(t, skillPath)
	if _, err := os.Stat(filepath.Join(installedPath, "agents")); !os.IsNotExist(err) {
		t.Fatalf("expected no agents directory, got err=%v", err)
	}
}

func TestInstallCodexSkillFallsBackToHomeWhenCodexHomeUnset(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CODEX_HOME", "")
	t.Setenv("HOME", tmp)

	installedPath, err := InstallCodexSkill(false)
	if err != nil {
		t.Fatalf("install codex skill: %v", err)
	}

	expected := filepath.Join(tmp, ".codex", "skills", skillDirName)
	if installedPath != expected {
		t.Fatalf("expected %s, got %s", expected, installedPath)
	}
}

func TestInstallCodexSkillExistingWithoutForceReusesDirectory(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CODEX_HOME", filepath.Join(tmp, "codex-home"))
	t.Setenv("HOME", tmp)

	firstPath, err := InstallCodexSkill(false)
	if err != nil {
		t.Fatalf("first install should succeed: %v", err)
	}

	secondPath, err := InstallCodexSkill(false)
	if err != nil {
		t.Fatalf("second install should succeed: %v", err)
	}
	if secondPath != firstPath {
		t.Fatalf("expected same install path, got %s and %s", firstPath, secondPath)
	}
}

func TestInstallCodexSkillForceOverwrites(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("CODEX_HOME", filepath.Join(tmp, "codex-home"))
	t.Setenv("HOME", tmp)

	installedPath, err := InstallCodexSkill(false)
	if err != nil {
		t.Fatalf("first install should succeed: %v", err)
	}

	customFile := filepath.Join(installedPath, "custom.txt")
	if err := os.WriteFile(customFile, []byte("custom"), 0o644); err != nil {
		t.Fatalf("write custom file: %v", err)
	}

	if _, err := InstallCodexSkill(true); err != nil {
		t.Fatalf("force install should succeed: %v", err)
	}

	if _, err := os.Stat(customFile); err == nil {
		t.Fatal("expected custom file removed by force overwrite")
	}
}

func TestInstallClaudeSkillCreatesOnlySkillFile(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	installedPath, err := InstallClaudeSkill(false)
	if err != nil {
		t.Fatalf("install claude skill: %v", err)
	}

	expectedPath := filepath.Join(tmp, ".claude", "skills", skillDirName)
	if installedPath != expectedPath {
		t.Fatalf("expected %s, got %s", expectedPath, installedPath)
	}

	skillPath := filepath.Join(installedPath, "SKILL.md")
	if _, err := os.Stat(skillPath); err != nil {
		t.Fatalf("missing SKILL.md: %v", err)
	}
	assertInstalledSharedSkill(t, skillPath)
	if _, err := os.Stat(filepath.Join(installedPath, "agents")); !os.IsNotExist(err) {
		t.Fatalf("expected no agents directory, got err=%v", err)
	}
}

func TestInstallCodexProviderGlobalCreatesSkillContextHooksAndConfig(t *testing.T) {
	tmp := t.TempDir()
	codexHome := filepath.Join(tmp, "codex-home")
	t.Setenv("CODEX_HOME", codexHome)
	t.Setenv("HOME", tmp)

	result, err := InstallProvider(InstallOptions{Provider: ProviderCodex})
	if err != nil {
		t.Fatalf("install codex provider: %v", err)
	}

	assertPathExists(t, filepath.Join(codexHome, "skills", skillDirName, "SKILL.md"))
	assertPathExists(t, filepath.Join(codexHome, "AGENTS.md"))
	assertPathExists(t, filepath.Join(codexHome, "hooks.json"))
	assertPathExists(t, filepath.Join(codexHome, "config.toml"))
	assertInstalledSharedSkill(t, filepath.Join(result.SkillPath, "SKILL.md"))
	assertFileContains(t, result.ContextPath, contextBlockBegin)
	assertFileContains(t, result.HookPath, sessionStartCommand)
	assertFileContains(t, result.CodexConfigPath, "[features]\ncodex_hooks = true")
	assertJSONFile(t, result.HookPath)
}

func TestInstallClaudeProviderGlobalCreatesSkillContextAndHooks(t *testing.T) {
	tmp := t.TempDir()
	t.Setenv("HOME", tmp)

	result, err := InstallProvider(InstallOptions{Provider: ProviderClaude})
	if err != nil {
		t.Fatalf("install claude provider: %v", err)
	}

	assertPathExists(t, filepath.Join(tmp, ".claude", "skills", skillDirName, "SKILL.md"))
	assertPathExists(t, filepath.Join(tmp, ".claude", "CLAUDE.md"))
	assertPathExists(t, filepath.Join(tmp, ".claude", "settings.json"))
	assertInstalledSharedSkill(t, filepath.Join(result.SkillPath, "SKILL.md"))
	assertFileContains(t, result.ContextPath, contextBlockBegin)
	assertFileContains(t, result.HookPath, sessionStartCommand)
	assertJSONFile(t, result.HookPath)
	if result.CodexConfigPath != "" {
		t.Fatalf("Claude install should not write Codex config: %+v", result)
	}
}

func TestInstallCodexProviderLocalCreatesRepoAssets(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", "")

	result, err := InstallProvider(InstallOptions{
		Provider:  ProviderCodex,
		Local:     true,
		LocalRoot: root,
	})
	if err != nil {
		t.Fatalf("install codex locally: %v", err)
	}

	if result.ContextPath != filepath.Join(root, "AGENTS.md") {
		t.Fatalf("unexpected context path: %s", result.ContextPath)
	}
	assertPathExists(t, filepath.Join(root, ".codex", "skills", skillDirName, "SKILL.md"))
	assertPathExists(t, filepath.Join(root, ".codex", "hooks.json"))
	assertPathExists(t, filepath.Join(root, ".codex", "config.toml"))
	assertFileContains(t, result.ContextPath, contextBlockBegin)
	assertFileContains(t, result.HookPath, sessionStartCommand)
	assertFileContains(t, result.CodexConfigPath, "codex_hooks = true")
}

func TestInstallClaudeProviderLocalUsesAgentsAndPointer(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())

	result, err := InstallProvider(InstallOptions{
		Provider:  ProviderClaude,
		Local:     true,
		LocalRoot: root,
	})
	if err != nil {
		t.Fatalf("install claude locally: %v", err)
	}

	if result.ContextPath != filepath.Join(root, "AGENTS.md") {
		t.Fatalf("unexpected context path: %s", result.ContextPath)
	}
	if result.ClaudePointerPath != filepath.Join(root, "CLAUDE.md") {
		t.Fatalf("unexpected Claude pointer path: %s", result.ClaudePointerPath)
	}
	assertPathExists(t, filepath.Join(root, ".claude", "skills", skillDirName, "SKILL.md"))
	assertPathExists(t, filepath.Join(root, ".claude", "settings.json"))
	assertFileContains(t, result.ContextPath, contextBlockBegin)
	assertFileContains(t, result.HookPath, sessionStartCommand)
	assertFileEquals(t, result.ClaudePointerPath, claudeLocalPointer)
}

func TestInstallLocalCodexAndClaudeShareOneContextBlock(t *testing.T) {
	root := t.TempDir()
	t.Setenv("HOME", t.TempDir())
	t.Setenv("CODEX_HOME", "")

	if _, err := InstallProvider(InstallOptions{Provider: ProviderCodex, Local: true, LocalRoot: root}); err != nil {
		t.Fatalf("install codex locally: %v", err)
	}
	if _, err := InstallProvider(InstallOptions{Provider: ProviderClaude, Local: true, LocalRoot: root}); err != nil {
		t.Fatalf("install claude locally: %v", err)
	}

	content := readFile(t, filepath.Join(root, "AGENTS.md"))
	if count := strings.Count(content, contextBlockBegin); count != 1 {
		t.Fatalf("expected one managed context block, got %d in:\n%s", count, content)
	}
}

func TestInstallHookConfigAtPathMergesAndIsIdempotent(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.json")
	existing := []byte(`{"hooks":{"SessionStart":[{"matcher":"startup","hooks":[{"type":"command","command":"echo existing"}]}]}}`)
	if err := os.WriteFile(path, existing, 0o644); err != nil {
		t.Fatalf("seed hooks: %v", err)
	}

	if _, err := InstallHookConfigAtPath(path); err != nil {
		t.Fatalf("first hook install: %v", err)
	}
	if _, err := InstallHookConfigAtPath(path); err != nil {
		t.Fatalf("second hook install: %v", err)
	}

	content := readFile(t, path)
	if count := strings.Count(content, sessionStartCommand); count != 1 {
		t.Fatalf("expected one treelines hook, got %d in:\n%s", count, content)
	}
	if count := strings.Count(content, "echo existing"); count != 1 {
		t.Fatalf("expected existing hook preserved, got %d in:\n%s", count, content)
	}
	assertJSONFile(t, path)
}

func TestInstallHookConfigAtPathRejectsInvalidJSON(t *testing.T) {
	path := filepath.Join(t.TempDir(), "hooks.json")
	if err := os.WriteFile(path, []byte("{invalid"), 0o644); err != nil {
		t.Fatalf("seed hooks: %v", err)
	}

	_, err := InstallHookConfigAtPath(path)
	if err == nil {
		t.Fatal("expected invalid JSON error")
	}
	if !strings.Contains(err.Error(), "parse current hook config") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}

func assertInstalledSharedSkill(t *testing.T, skillPath string) {
	t.Helper()

	installedSkill, err := os.ReadFile(skillPath)
	if err != nil {
		t.Fatalf("read installed SKILL.md: %v", err)
	}
	expectedSkill, err := bundledFiles.ReadFile(bundledSkillPath)
	if err != nil {
		t.Fatalf("read bundled SKILL.md: %v", err)
	}
	if string(installedSkill) != string(expectedSkill) {
		t.Fatal("expected installed skill content to match bundled shared skill content")
	}
}

func assertPathExists(t *testing.T, path string) {
	t.Helper()
	if _, err := os.Stat(path); err != nil {
		t.Fatalf("expected path %s to exist: %v", path, err)
	}
}

func assertFileContains(t *testing.T, path string, substring string) {
	t.Helper()
	content := readFile(t, path)
	if !strings.Contains(content, substring) {
		t.Fatalf("expected %s to contain %q, got:\n%s", path, substring, content)
	}
}

func assertFileEquals(t *testing.T, path string, expected string) {
	t.Helper()
	content := readFile(t, path)
	if content != expected {
		t.Fatalf("expected %s to equal %q, got %q", path, expected, content)
	}
}

func assertJSONFile(t *testing.T, path string) {
	t.Helper()
	var decoded any
	if err := json.Unmarshal([]byte(readFile(t, path)), &decoded); err != nil {
		t.Fatalf("expected %s to be valid JSON: %v", path, err)
	}
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	content, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(content)
}
