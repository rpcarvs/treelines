package scanner

import (
	"os"
	"path/filepath"
	"testing"
)

func TestScanAll_SkipsAnchoredIgnoredTargetDir(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, ".gitignore"), []byte("/target\n"), 0o644); err != nil {
		t.Fatalf("write .gitignore: %v", err)
	}

	mustMkdirAll(t, filepath.Join(root, "src"))
	mustMkdirAll(t, filepath.Join(root, "target", "debug"))

	mustWriteFile(t, filepath.Join(root, "src", "main.rs"), "fn main() {}\n")
	mustWriteFile(t, filepath.Join(root, "target", "debug", "build.rs"), "fn main() {}\n")

	sc := NewScanner(root)
	files, err := sc.ScanAll()
	if err != nil {
		t.Fatalf("scan all: %v", err)
	}

	if len(files) != 1 {
		t.Fatalf("expected 1 source file, got %d", len(files))
	}
	if files[0].RelPath != "src/main.rs" {
		t.Fatalf("expected src/main.rs, got %s", files[0].RelPath)
	}
}

func mustMkdirAll(t *testing.T, path string) {
	t.Helper()
	if err := os.MkdirAll(path, 0o755); err != nil {
		t.Fatalf("mkdir %s: %v", path, err)
	}
}

func mustWriteFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
