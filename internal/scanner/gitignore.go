package scanner

import (
	"bufio"
	"os"
	"path/filepath"
	"strings"
)

// Matcher loads .gitignore patterns and checks paths against them.
type Matcher struct {
	root     string
	patterns []pattern
}

type pattern struct {
	negation bool
	glob     string
}

// NewMatcher creates a Matcher by loading .gitignore from the given root directory.
func NewMatcher(root string) *Matcher {
	m := &Matcher{root: root}
	m.loadPatterns()
	return m
}

// loadPatterns reads and parses .gitignore patterns from disk.
func (m *Matcher) loadPatterns() {
	path := filepath.Join(m.root, ".gitignore")
	f, err := os.Open(path)
	if err != nil {
		return
	}
	defer func() { _ = f.Close() }()

	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		m.patterns = append(m.patterns, parsePattern(line))
	}
}

// parsePattern parses a single gitignore line into a pattern struct.
func parsePattern(raw string) pattern {
	p := pattern{}

	if strings.HasPrefix(raw, "!") {
		p.negation = true
		raw = raw[1:]
	}

	raw = strings.TrimSuffix(raw, "/")
	p.glob = raw
	return p
}

// IsIgnored returns true if the given path (relative to root) should be ignored.
func (m *Matcher) IsIgnored(relPath string) bool {
	relPath = filepath.ToSlash(relPath)

	if isAlwaysIgnored(relPath) {
		return true
	}

	ignored := false
	for _, p := range m.patterns {
		if p.matches(relPath) {
			ignored = !p.negation
		}
	}
	return ignored
}

// isAlwaysIgnored returns true for paths that are always excluded (like .git).
func isAlwaysIgnored(relPath string) bool {
	parts := strings.Split(relPath, "/")
	for _, part := range parts {
		if part == ".git" || part == ".treelines" {
			return true
		}
	}
	return false
}

func (p *pattern) matches(relPath string) bool {
	segments := strings.Split(relPath, "/")

	for i := range segments {
		candidate := strings.Join(segments[i:], "/")
		if matchGlob(p.glob, candidate) {
			return true
		}
		if matchGlob(p.glob, segments[i]) {
			return true
		}
	}

	return false
}

// matchGlob performs a simple glob match supporting * wildcards.
func matchGlob(pattern, name string) bool {
	matched, _ := filepath.Match(pattern, name)
	return matched
}
