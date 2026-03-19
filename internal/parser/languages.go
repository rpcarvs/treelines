package parser

import (
	"github.com/rpcarvs/treelines/internal/model"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
	tree_sitter_go "github.com/tree-sitter/tree-sitter-go/bindings/go"
	tree_sitter_python "github.com/tree-sitter/tree-sitter-python/bindings/go"
	tree_sitter_rust "github.com/tree-sitter/tree-sitter-rust/bindings/go"
)

// GetLanguage returns the tree-sitter Language for the given language name.
// Returns nil if the language is not supported.
func GetLanguage(name string) *tree_sitter.Language {
	switch name {
	case model.LangPython:
		return tree_sitter.NewLanguage(tree_sitter_python.Language())
	case model.LangGo:
		return tree_sitter.NewLanguage(tree_sitter_go.Language())
	case model.LangRust:
		return tree_sitter.NewLanguage(tree_sitter_rust.Language())
	default:
		return nil
	}
}
