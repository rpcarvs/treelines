package scanner

import (
	"io/fs"
	"path/filepath"
	"strings"

	"lines/internal/model"
)

var extToLang = map[string]string{
	".py": model.LangPython,
	".go": model.LangGo,
	".rs": model.LangRust,
}

// FileInfo holds metadata about a discovered source file.
type FileInfo struct {
	Path     string
	Language string
	RelPath  string
}

// Scanner walks a directory tree to find supported source files.
type Scanner struct {
	root    string
	matcher *Matcher
}

// NewScanner creates a Scanner for the given root directory.
func NewScanner(root string) *Scanner {
	return &Scanner{
		root:    root,
		matcher: NewMatcher(root),
	}
}

// ScanAll walks the directory tree and returns all supported source files,
// skipping ignored paths and directories starting with '.' or '_'.
func (s *Scanner) ScanAll() ([]FileInfo, error) {
	var files []FileInfo

	err := filepath.WalkDir(s.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		relPath, err := filepath.Rel(s.root, path)
		if err != nil {
			return err
		}

		if d.IsDir() {
			name := d.Name()
			if name != "." && (strings.HasPrefix(name, ".") || strings.HasPrefix(name, "_")) {
				return filepath.SkipDir
			}
			if s.matcher.IsIgnored(relPath) {
				return filepath.SkipDir
			}
			return nil
		}

		if s.matcher.IsIgnored(relPath) {
			return nil
		}

		ext := filepath.Ext(path)
		lang, ok := extToLang[ext]
		if !ok {
			return nil
		}

		files = append(files, FileInfo{
			Path:     path,
			Language: lang,
			RelPath:  relPath,
		})
		return nil
	})

	return files, err
}
