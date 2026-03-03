package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMatcher_IsIgnored_TargetPatternAnchored(t *testing.T) {
	root := t.TempDir()
	writeGitignore(t, root, "/target\n")

	m := NewMatcher(root)

	if !m.IsIgnored("target/release/app") {
		t.Fatalf("expected anchored /target to ignore target subtree")
	}
	if m.IsIgnored("src/target/release/app") {
		t.Fatalf("expected anchored /target to not ignore nested src/target")
	}
}

func TestMatcher_IsIgnored_TargetPatternDirectory(t *testing.T) {
	root := t.TempDir()
	writeGitignore(t, root, "target/\n")

	m := NewMatcher(root)

	if !m.IsIgnored("target/debug/lib.rs") {
		t.Fatalf("expected target/ to ignore target subtree")
	}
	if !m.IsIgnored("foo/target/debug/lib.rs") {
		t.Fatalf("expected unanchored target/ to ignore nested target subtree")
	}
}

func TestMatcher_IsIgnored_TargetPatternLiteral(t *testing.T) {
	root := t.TempDir()
	writeGitignore(t, root, "target\n")

	m := NewMatcher(root)

	if !m.IsIgnored("target/debug/lib.rs") {
		t.Fatalf("expected target to ignore target subtree")
	}
	if !m.IsIgnored("foo/target/debug/lib.rs") {
		t.Fatalf("expected unanchored target to ignore nested target subtree")
	}
}

func writeGitignore(t *testing.T, root, contents string) {
	t.Helper()

	path := filepath.Join(root, ".gitignore")
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}
}
