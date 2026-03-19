package extractor

import (
	"fmt"

	"github.com/rpcarvs/treelines/internal/model"
	"github.com/rpcarvs/treelines/internal/parser"
)

// ExtractPythonAll parses a Python file and returns static __all__ names and assignment line.
func ExtractPythonAll(result *parser.ParseResult) ([]string, int, bool, error) {
	if result == nil || result.Tree == nil {
		return nil, 0, false, fmt.Errorf("nil parse result")
	}
	queryStr, err := loadQuery(model.LangPython)
	if err != nil {
		return nil, 0, false, err
	}
	tsLang := getLanguage(model.LangPython)
	root := result.Tree.RootNode()
	matches, captureNames, err := runQuery(queryStr, tsLang, root, result.Source)
	if err != nil {
		return nil, 0, false, err
	}
	names, line, hasLine := extractPythonAllNamesAndLine(matches, captureNames, result.Source)
	return names, line, hasLine, nil
}
