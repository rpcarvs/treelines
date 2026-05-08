package skillinstaller

import (
	"embed"
	"errors"
	"fmt"
	"os"
	"path/filepath"
)

const skillDirName = "treelines-codebase-exploration"
const bundledSkillPath = "bundled/treelines-codebase-exploration/SKILL.md"

// bundledFiles contains built-in skill files to install for supported tools.
//
//go:embed bundled/treelines-codebase-exploration/SKILL.md
var bundledFiles embed.FS

// Provider identifies a supported agent integration target.
type Provider string

const (
	ProviderCodex  Provider = "codex"
	ProviderClaude Provider = "claude"
)

// InstallOptions configures a provider-level agent installation.
type InstallOptions struct {
	Provider  Provider
	Local     bool
	LocalRoot string
	Force     bool
}

// InstallResult reports all paths touched by a provider install.
type InstallResult struct {
	SkillPath           string
	ContextPath         string
	ContextAction       string
	HookPath            string
	HookAction          string
	ClaudePointerPath   string
	ClaudePointerAction string
}

// InstallProvider installs skill, context, and hooks for one supported agent.
func InstallProvider(options InstallOptions) (InstallResult, error) {
	if err := validateInstallOptions(options); err != nil {
		return InstallResult{}, err
	}

	skillRoot, err := skillsRoot(options)
	if err != nil {
		return InstallResult{}, err
	}
	contextPath, err := contextPath(options)
	if err != nil {
		return InstallResult{}, err
	}
	hookPath, err := hookConfigPath(options)
	if err != nil {
		return InstallResult{}, err
	}

	files := map[string]string{"SKILL.md": bundledSkillPath}
	skillPath, err := installBundledSkill(skillRoot, files, options.Force)
	if err != nil {
		return InstallResult{}, err
	}
	contextAction, err := InstallContextAtPath(contextPath)
	if err != nil {
		return InstallResult{}, err
	}
	hookAction, err := InstallHookConfigAtPath(hookPath)
	if err != nil {
		return InstallResult{}, err
	}

	result := InstallResult{
		SkillPath:     skillPath,
		ContextPath:   contextPath,
		ContextAction: contextAction,
		HookPath:      hookPath,
		HookAction:    hookAction,
	}

	if options.Provider == ProviderClaude && options.Local {
		pointerPath := filepath.Join(options.LocalRoot, "CLAUDE.md")
		action, err := InstallClaudePointerAtPath(pointerPath)
		if err != nil {
			return InstallResult{}, err
		}
		result.ClaudePointerPath = pointerPath
		result.ClaudePointerAction = action
	}

	return result, nil
}

// InstallCodexSkill installs the bundled treelines skill into Codex skills directory.
func InstallCodexSkill(force bool) (string, error) {
	root, err := codexSkillsRoot()
	if err != nil {
		return "", err
	}

	files := map[string]string{"SKILL.md": bundledSkillPath}

	return installBundledSkill(root, files, force)
}

// InstallClaudeSkill installs the bundled treelines skill into Claude skills directory.
func InstallClaudeSkill(force bool) (string, error) {
	root, err := claudeSkillsRoot()
	if err != nil {
		return "", err
	}

	files := map[string]string{"SKILL.md": bundledSkillPath}

	return installBundledSkill(root, files, force)
}

// codexSkillsRoot resolves the target Codex skills root directory.
func codexSkillsRoot() (string, error) {
	codexHome := os.Getenv("CODEX_HOME")
	if codexHome == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return "", fmt.Errorf("resolve home directory: %w", err)
		}
		codexHome = filepath.Join(home, ".codex")
	}
	return filepath.Join(codexHome, "skills"), nil
}

// claudeSkillsRoot resolves the target Claude skills root directory.
func claudeSkillsRoot() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	return filepath.Join(home, ".claude", "skills"), nil
}

// validateInstallOptions verifies provider and scope before writing files.
func validateInstallOptions(options InstallOptions) error {
	switch options.Provider {
	case ProviderCodex, ProviderClaude:
	default:
		return fmt.Errorf("unsupported install provider %q", options.Provider)
	}
	if options.Local && options.LocalRoot == "" {
		return fmt.Errorf("local install requires repository root")
	}
	return nil
}

// skillsRoot resolves the target skill root for a provider install.
func skillsRoot(options InstallOptions) (string, error) {
	if options.Local {
		switch options.Provider {
		case ProviderCodex:
			return filepath.Join(options.LocalRoot, ".codex", "skills"), nil
		case ProviderClaude:
			return filepath.Join(options.LocalRoot, ".claude", "skills"), nil
		}
	}

	switch options.Provider {
	case ProviderCodex:
		return codexSkillsRoot()
	case ProviderClaude:
		return claudeSkillsRoot()
	default:
		return "", fmt.Errorf("unsupported install provider %q", options.Provider)
	}
}

// contextPath resolves where the managed codebase exploration context is installed.
func contextPath(options InstallOptions) (string, error) {
	if options.Local {
		return filepath.Join(options.LocalRoot, "AGENTS.md"), nil
	}

	switch options.Provider {
	case ProviderCodex:
		return CodexContextPath()
	case ProviderClaude:
		return ClaudeContextPath()
	default:
		return "", fmt.Errorf("unsupported install provider %q", options.Provider)
	}
}

// hookConfigPath resolves where provider hook configuration is installed.
func hookConfigPath(options InstallOptions) (string, error) {
	if options.Local {
		switch options.Provider {
		case ProviderCodex:
			return filepath.Join(options.LocalRoot, ".codex", "hooks.json"), nil
		case ProviderClaude:
			return filepath.Join(options.LocalRoot, ".claude", "settings.json"), nil
		}
	}

	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve home directory: %w", err)
	}
	switch options.Provider {
	case ProviderCodex:
		codexHome := os.Getenv("CODEX_HOME")
		if codexHome == "" {
			codexHome = filepath.Join(home, ".codex")
		}
		return filepath.Join(codexHome, "hooks.json"), nil
	case ProviderClaude:
		return filepath.Join(home, ".claude", "settings.json"), nil
	default:
		return "", fmt.Errorf("unsupported install provider %q", options.Provider)
	}
}

// installBundledSkill writes embedded files into the destination skill directory.
func installBundledSkill(root string, files map[string]string, force bool) (string, error) {
	target := filepath.Join(root, skillDirName)
	if err := ensureTarget(target, force); err != nil {
		return "", err
	}

	for relPath, bundledPath := range files {
		content, err := bundledFiles.ReadFile(bundledPath)
		if err != nil {
			return "", fmt.Errorf("read bundled file %s: %w", bundledPath, err)
		}

		filePath := filepath.Join(target, relPath)
		if err := os.MkdirAll(filepath.Dir(filePath), 0o755); err != nil {
			return "", fmt.Errorf("create directory %s: %w", filepath.Dir(filePath), err)
		}
		if err := os.WriteFile(filePath, content, 0o644); err != nil {
			return "", fmt.Errorf("write file %s: %w", filePath, err)
		}
	}

	return target, nil
}

// ensureTarget verifies destination state and removes existing data when forced.
func ensureTarget(target string, force bool) error {
	_, err := os.Stat(target)
	if err == nil && force {
		if err := os.RemoveAll(target); err != nil {
			return fmt.Errorf("remove existing skill at %s: %w", target, err)
		}
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("check target %s: %w", target, err)
	}

	if err := os.MkdirAll(target, 0o755); err != nil {
		return fmt.Errorf("create skill directory %s: %w", target, err)
	}
	return nil
}
