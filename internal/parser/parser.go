package parser

import (
	"fmt"
	"os"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ParseResult holds the tree-sitter parse tree, source bytes, and file metadata.
type ParseResult struct {
	Tree     *tree_sitter.Tree
	Source   []byte
	Language string
	Path     string
}

// Parser wraps a tree-sitter parser for reuse across multiple files.
type Parser struct {
	inner *tree_sitter.Parser
}

// NewParser creates a new tree-sitter parser instance.
func NewParser() *Parser {
	return &Parser{inner: tree_sitter.NewParser()}
}

// ParseFile reads a file, configures the parser for the given language,
// and returns the parsed tree along with the source bytes.
func (p *Parser) ParseFile(absPath, storePath, lang string) (*ParseResult, error) {
	tsLang := GetLanguage(lang)
	if tsLang == nil {
		return nil, fmt.Errorf("unsupported language: %s", lang)
	}

	if err := p.inner.SetLanguage(tsLang); err != nil {
		return nil, fmt.Errorf("set language: %w", err)
	}

	src, err := os.ReadFile(absPath)
	if err != nil {
		return nil, fmt.Errorf("read file: %w", err)
	}

	tree := p.inner.Parse(src, nil)

	return &ParseResult{
		Tree:     tree,
		Source:   src,
		Language: lang,
		Path:     storePath,
	}, nil
}

// Close releases resources held by the underlying tree-sitter parser.
func (p *Parser) Close() {
	p.inner.Close()
}
