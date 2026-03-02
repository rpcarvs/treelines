package extractor

import (
	"fmt"
	"regexp"
	"strings"

	"lines/internal/model"
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

// copyMatch creates a deep copy of a tree-sitter QueryMatch.
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

// findEnclosingElement walks up from a node to find the nearest ancestor
// whose kind matches one of the given node types.
func findEnclosingElement(node *tree_sitter.Node, kinds []string) *tree_sitter.Node {
	current := node.Parent()
	for current != nil {
		k := current.Kind()
		for _, want := range kinds {
			if k == want {
				return current
			}
		}
		current = current.Parent()
	}
	return nil
}

// extractCallEdges builds CALLS edges from @call_name captures. It finds the
// enclosing function for each call, resolves the target name, and produces
// an edge when both caller and callee are known. For qualified calls like
// pkg.Function() or module::func(), it extracts the qualifier from the
// tree-sitter node and attempts FQName-based resolution.
func extractCallEdges(
	matches []*tree_sitter.QueryMatch,
	captureNames []string,
	source []byte,
	enclosingKinds []string,
	elementsByNode map[nodeKey]string,
	elementsByID map[string]model.Element,
	localResolver *Resolver,
	globalResolver *Resolver,
) []model.Edge {
	var edges []model.Edge
	hintsByEnclosing := make(map[nodeKey]map[string]string)
	for _, m := range matches {
		caps := captureMap(m, captureNames)
		callNameNode, ok := caps["call_name"]
		if !ok {
			continue
		}

		enclosing := findEnclosingElement(callNameNode, enclosingKinds)
		if enclosing == nil {
			continue
		}
		key := makeNodeKey(enclosing)
		callerID, ok := elementsByNode[key]
		if !ok {
			continue
		}

		calleeName := nodeText(callNameNode, source)
		callerElem, hasCaller := elementsByID[callerID]

		var targetID string
		var found bool
		qualifier := extractCallQualifier(callNameNode, source)
		if qualifier != "" {
			targetID, found = globalResolver.ResolveQualified(qualifier, calleeName)
			if !found && hasCaller {
				hints, ok := hintsByEnclosing[key]
				if !ok {
					hints = qualifierHintsForCall(enclosing, source, callerElem)
					hintsByEnclosing[key] = hints
				}
				if mappedQualifier, ok := hints[qualifier]; ok && mappedQualifier != "" {
					targetID, found = globalResolver.ResolveQualified(mappedQualifier, calleeName)
				}
			}
		} else {
			targetID, found = localResolver.Resolve(calleeName)
		}
		if !found {
			continue
		}

		if callerID == targetID {
			continue
		}
		edges = append(edges, model.Edge{
			From: callerID,
			To:   targetID,
			Type: model.EdgeCalls,
		})
	}
	return edges
}

var (
	goShortVarTypeRe = regexp.MustCompile(`\b([A-Za-z_]\w*)\s*:=\s*&?([A-Za-z_]\w*)\s*\{`)
	goVarDeclTypeRe  = regexp.MustCompile(`\bvar\s+([A-Za-z_]\w*)\s+\*?([A-Za-z_]\w*)\b`)
	goNewTypeCallRe  = regexp.MustCompile(`\b([A-Za-z_]\w*)\s*:=\s*(?:([A-Za-z_]\w*)\.)?New([A-Za-z_]\w*)\s*\(`)
)

// qualifierHintsForCall builds deterministic qualifier substitutions for receiver/self
// and simple local variable type hints within the enclosing function/method.
func qualifierHintsForCall(enclosing *tree_sitter.Node, source []byte, caller model.Element) map[string]string {
	hints := make(map[string]string)

	switch caller.Language {
	case model.LangGo:
		if caller.Kind == model.KindMethod {
			if idx := strings.LastIndex(caller.FQName, "."); idx > 0 {
				receiverQual := caller.FQName[:idx]
				if receiver := enclosing.ChildByFieldName("receiver"); receiver != nil {
					text := nodeText(receiver, source)
					text = strings.TrimPrefix(text, "(")
					text = strings.TrimSuffix(text, ")")
					parts := strings.Fields(text)
					if len(parts) >= 2 {
						hints[parts[0]] = receiverQual
					}
				}
			}
		}
		pkg := goPackageFromFQName(caller.FQName)
		if pkg == "" {
			return hints
		}
		text := nodeText(enclosing, source)
		for _, m := range goShortVarTypeRe.FindAllStringSubmatch(text, -1) {
			if len(m) == 3 {
				hints[m[1]] = pkg + "." + m[2]
			}
		}
		for _, m := range goVarDeclTypeRe.FindAllStringSubmatch(text, -1) {
			if len(m) == 3 {
				hints[m[1]] = pkg + "." + m[2]
			}
		}
	for _, m := range goNewTypeCallRe.FindAllStringSubmatch(text, -1) {
		if len(m) == 4 {
			typePkg := pkg
			if m[2] != "" {
				typePkg = m[2]
			}
			hints[m[1]] = typePkg + "." + m[3]
		}
	}

	case model.LangPython:
		if caller.Kind == model.KindMethod {
			if idx := strings.LastIndex(caller.FQName, "."); idx > 0 {
				hints["self"] = caller.FQName[:idx]
			}
		}

	case model.LangRust:
		if caller.Kind == model.KindMethod {
			if idx := strings.LastIndex(caller.FQName, "::"); idx > 0 {
				hints["self"] = caller.FQName[:idx]
			}
		}
	}

	return hints
}

// goPackageFromFQName returns the package segment from a Go element FQName.
func goPackageFromFQName(fq string) string {
	if idx := strings.Index(fq, "."); idx > 0 {
		return fq[:idx]
	}
	return ""
}

// buildElementsByID creates a lookup map from element ID to element metadata.
func buildElementsByID(elements []model.Element) map[string]model.Element {
	byID := make(map[string]model.Element, len(elements))
	for _, el := range elements {
		byID[el.ID] = el
	}
	return byID
}

// extractCallQualifier extracts the object/package qualifier from a qualified
// call expression. For example, from `pkg.Function()` it returns "pkg".
// It handles attribute (Python), selector_expression (Go), and
// field_expression (Rust) parent nodes.
func extractCallQualifier(callNameNode *tree_sitter.Node, source []byte) string {
	parent := callNameNode.Parent()
	if parent == nil {
		return ""
	}
	kind := parent.Kind()
	switch kind {
	case "attribute":
		obj := parent.ChildByFieldName("object")
		if obj != nil {
			return nodeText(obj, source)
		}
	case "selector_expression":
		operand := parent.ChildByFieldName("operand")
		if operand != nil {
			return nodeText(operand, source)
		}
	case "field_expression":
		value := parent.ChildByFieldName("value")
		if value != nil {
			return nodeText(value, source)
		}
	}
	return ""
}

// nodeKey identifies a tree-sitter node by its start/end byte offsets.
type nodeKey struct {
	startByte uint
	endByte   uint
}

// makeNodeKey creates a nodeKey from a tree-sitter node's byte offsets.
func makeNodeKey(node *tree_sitter.Node) nodeKey {
	return nodeKey{
		startByte: uint(node.StartByte()),
		endByte:   uint(node.EndByte()),
	}
}
