package extractor

import (
	"embed"
	"github.com/rpcarvs/treelines/internal/model"
	"github.com/rpcarvs/treelines/internal/parser"
)

//go:embed queries/*.scm
var queryFiles embed.FS

// ExtractionResult holds the elements and edges extracted from a parsed file.
type ExtractionResult struct {
	Elements []model.Element
	Edges    []model.Edge
}

// Extractor defines the interface for language-specific code extraction.
type Extractor interface {
	Extract(result *parser.ParseResult) (*ExtractionResult, error)
}

// ForLanguage returns the appropriate Extractor for the given language.
// Returns nil if the language is not supported.
func ForLanguage(lang string) Extractor {
	switch lang {
	case model.LangPython:
		return &PythonExtractor{}
	case model.LangGo:
		return &GoExtractor{}
	case model.LangRust:
		return &RustExtractor{}
	default:
		return nil
	}
}
