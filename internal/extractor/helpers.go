package extractor

import (
	"fmt"
	"strings"

	"lines/internal/parser"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// nodeText extracts the UTF-8 text of a tree-sitter node from the source.
func nodeText(node *tree_sitter.Node, source []byte) string {
	return node.Utf8Text(source)
}

// extractDocstring returns the docstring associated with a node,
// using language-specific conventions to locate it.
func extractDocstring(node *tree_sitter.Node, source []byte, lang string) string {
	switch lang {
	case "python":
		return extractPythonDocstring(node, source)
	case "go":
		return extractGoDocstring(node, source)
	case "rust":
		return extractRustDocstring(node, source)
	default:
		return ""
	}
}

func extractPythonDocstring(node *tree_sitter.Node, source []byte) string {
	body := node.ChildByFieldName("body")
	if body == nil || body.ChildCount() == 0 {
		return ""
	}
	first := body.Child(0)
	if first == nil || first.Kind() != "expression_statement" {
		return ""
	}
	if first.ChildCount() == 0 {
		return ""
	}
	strNode := first.Child(0)
	if strNode == nil || strNode.Kind() != "string" {
		return ""
	}
	text := nodeText(strNode, source)
	text = strings.TrimPrefix(text, `"""`)
	text = strings.TrimSuffix(text, `"""`)
	text = strings.TrimPrefix(text, `'''`)
	text = strings.TrimSuffix(text, `'''`)
	text = strings.TrimPrefix(text, `"`)
	text = strings.TrimSuffix(text, `"`)
	text = strings.TrimPrefix(text, `'`)
	text = strings.TrimSuffix(text, `'`)
	return strings.TrimSpace(text)
}

func extractGoDocstring(node *tree_sitter.Node, source []byte) string {
	var lines []string
	sibling := node.PrevSibling()
	for sibling != nil && sibling.Kind() == "comment" {
		lines = append([]string{nodeText(sibling, source)}, lines...)
		sibling = sibling.PrevSibling()
	}
	if len(lines) == 0 {
		return ""
	}
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimPrefix(line, "//")
		line = strings.TrimSpace(line)
		cleaned = append(cleaned, line)
	}
	return strings.Join(cleaned, "\n")
}

func extractRustDocstring(node *tree_sitter.Node, source []byte) string {
	var lines []string
	sibling := node.PrevSibling()
	for sibling != nil {
		kind := sibling.Kind()
		if kind != "line_comment" && kind != "block_comment" {
			break
		}
		text := nodeText(sibling, source)
		if !strings.HasPrefix(text, "///") && !strings.HasPrefix(text, "//!") {
			break
		}
		lines = append([]string{text}, lines...)
		sibling = sibling.PrevSibling()
	}
	if len(lines) == 0 {
		return ""
	}
	var cleaned []string
	for _, line := range lines {
		line = strings.TrimPrefix(line, "///")
		line = strings.TrimPrefix(line, "//!")
		line = strings.TrimSpace(line)
		cleaned = append(cleaned, line)
	}
	return strings.Join(cleaned, "\n")
}

// lineCount returns the number of source lines spanned by a node.
func lineCount(node *tree_sitter.Node) int {
	return int(node.EndPosition().Row-node.StartPosition().Row) + 1
}

// signatureLine returns the first line of the node's text.
func signatureLine(node *tree_sitter.Node, source []byte) string {
	text := nodeText(node, source)
	if idx := strings.IndexByte(text, '\n'); idx >= 0 {
		return text[:idx]
	}
	return text
}

// loadQuery reads a query file for the given language from the embedded FS.
func loadQuery(lang string) ([]byte, error) {
	path := fmt.Sprintf("queries/%s.scm", lang)
	data, err := queryFiles.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("load query %s: %w", lang, err)
	}
	return data, nil
}

// runQuery compiles and executes a tree-sitter query, returning all matches
// and the capture names. Returns an error if query compilation fails.
func runQuery(queryStr []byte, tsLang *tree_sitter.Language, root *tree_sitter.Node, source []byte) ([]*tree_sitter.QueryMatch, []string, error) {
	query, qErr := tree_sitter.NewQuery(tsLang, string(queryStr))
	if qErr != nil {
		return nil, nil, fmt.Errorf("compile query: %s", qErr.Error())
	}
	defer query.Close()

	cursor := tree_sitter.NewQueryCursor()
	defer cursor.Close()

	matches := cursor.Matches(query, root, source)
	captureNames := query.CaptureNames()

	var result []*tree_sitter.QueryMatch
	for {
		m := matches.Next()
		if m == nil {
			break
		}
		copied := copyMatch(m)
		result = append(result, copied)
	}
	return result, captureNames, nil
}

func copyMatch(m *tree_sitter.QueryMatch) *tree_sitter.QueryMatch {
	captures := make([]tree_sitter.QueryCapture, len(m.Captures))
	copy(captures, m.Captures)
	return &tree_sitter.QueryMatch{
		Captures:     captures,
		PatternIndex: m.PatternIndex,
	}
}

// captureMap builds a map from capture name to the first matching node
// for a single query match.
func captureMap(m *tree_sitter.QueryMatch, names []string) map[string]*tree_sitter.Node {
	result := make(map[string]*tree_sitter.Node, len(m.Captures))
	for _, c := range m.Captures {
		idx := int(c.Index)
		if idx < len(names) {
			if _, exists := result[names[idx]]; !exists {
				node := c.Node
				result[names[idx]] = &node
			}
		}
	}
	return result
}

// getLanguage returns the tree-sitter Language for the given language name.
func getLanguage(lang string) *tree_sitter.Language {
	return parser.GetLanguage(lang)
}
