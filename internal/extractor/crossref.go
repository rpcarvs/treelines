package extractor

import (
	"path/filepath"

	"lines/internal/model"
	"lines/internal/parser"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ResolveCrossPackageCalls re-parses indexed files and resolves call
// expressions against all known elements. Returns CALLS edges with
// cross-package visibility.
func ResolveCrossPackageCalls(allElements []model.Element, p *parser.Parser, root string) []model.Edge {
	globalResolver := NewResolver(allElements)

	type fileGroup struct {
		lang     string
		elements []model.Element
	}
	byFile := make(map[string]*fileGroup)
	for _, el := range allElements {
		fg, ok := byFile[el.Path]
		if !ok {
			fg = &fileGroup{lang: el.Language}
			byFile[el.Path] = fg
		}
		fg.elements = append(fg.elements, el)
	}

	// For Go, build per-package element lists (all files in same dir share a package).
	// For Python/Rust, each file is its own module scope.
	pkgElements := make(map[string][]model.Element)
	for path, fg := range byFile {
		if fg.lang == model.LangGo {
			dir := filepath.Dir(path)
			pkgElements[dir] = append(pkgElements[dir], fg.elements...)
		}
	}

	var allEdges []model.Edge
	seen := make(map[string]bool)

	for path, fg := range byFile {
		absPath := filepath.Join(root, path)
		result, err := p.ParseFile(absPath, path, fg.lang)
		if err != nil {
			continue
		}

		queryStr, err := loadQuery(fg.lang)
		if err != nil {
			result.Tree.Close()
			continue
		}

		tsLang := getLanguage(fg.lang)
		tsRoot := result.Tree.RootNode()
		matches, captureNames, err := runQuery(queryStr, tsLang, tsRoot, result.Source)
		if err != nil {
			result.Tree.Close()
			continue
		}

		enclosingKinds := enclosingKindsForLang(fg.lang)
		elementsByNode := mapElementsToNodes(matches, captureNames, result.Source, fg.elements, enclosingKinds)

		// For Go, use package-scoped resolver so same-package cross-file calls resolve.
		localElements := fg.elements
		if fg.lang == model.LangGo {
			dir := filepath.Dir(path)
			localElements = pkgElements[dir]
		}
		localResolver := NewResolver(localElements)

		callEdges := extractCallEdges(matches, captureNames, result.Source, enclosingKinds, elementsByNode, localResolver, globalResolver)
		for _, e := range callEdges {
			key := e.From + "|" + e.To
			if !seen[key] {
				seen[key] = true
				allEdges = append(allEdges, e)
			}
		}

		result.Tree.Close()
	}

	return allEdges
}

// mapElementsToNodes matches extracted elements back to their tree-sitter
// nodes by comparing name and start line from query captures.
func mapElementsToNodes(
	matches []*tree_sitter.QueryMatch,
	captureNames []string,
	source []byte,
	fileElements []model.Element,
	enclosingKinds []string,
) map[nodeKey]string {
	type elemKey struct {
		name      string
		startLine int
	}
	lookup := make(map[elemKey]string)
	for _, el := range fileElements {
		lookup[elemKey{name: el.Name, startLine: el.StartLine}] = el.ID
	}

	result := make(map[nodeKey]string)
	for _, m := range matches {
		caps := captureMap(m, captureNames)
		elementNode, hasElement := caps["element"]
		nameNode, hasName := caps["name"]
		if !hasElement || !hasName {
			continue
		}
		if !isEnclosingKind(elementNode.Kind(), enclosingKinds) {
			continue
		}
		name := nodeText(nameNode, source)
		startLine := int(elementNode.StartPosition().Row) + 1
		if id, ok := lookup[elemKey{name: name, startLine: startLine}]; ok {
			result[makeNodeKey(elementNode)] = id
		}
	}
	return result
}

func enclosingKindsForLang(lang string) []string {
	switch lang {
	case model.LangGo:
		return []string{"function_declaration", "method_declaration"}
	case model.LangPython:
		return []string{"function_definition"}
	case model.LangRust:
		return []string{"function_item"}
	default:
		return nil
	}
}

func isEnclosingKind(kind string, kinds []string) bool {
	for _, k := range kinds {
		if kind == k {
			return true
		}
	}
	return false
}
