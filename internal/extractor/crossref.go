package extractor

import (
	"path/filepath"

	"lines/internal/model"
	"lines/internal/parser"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ResolveCrossPackageCalls re-parses indexed files and resolves
// cross-file edges against all known elements. It emits CALLS and
// Python IMPORTS/EXPORTS edges.
func ResolveCrossPackageCalls(allElements []model.Element, p *parser.Parser, root string) []model.Edge {
	globalResolver := NewResolver(allElements)
	allByID := buildElementsByID(allElements)

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
		elementsByID := buildElementsByID(fg.elements)
		var pythonImports *pythonImportMaps
		var pythonExports []string
		if fg.lang == model.LangPython {
			pythonImports = extractPythonImportMaps(matches, captureNames, result.Source, moduleNameFromElements(fg.elements))
			pythonExports = extractPythonAllNames(matches, captureNames, result.Source)
		}

		callEdges := extractCallEdges(matches, captureNames, result.Source, enclosingKinds, elementsByNode, elementsByID, localResolver, globalResolver, pythonImports)
		for _, e := range callEdges {
			key := e.From + "|" + e.To
			if !seen[key] {
				seen[key] = true
				allEdges = append(allEdges, e)
			}
		}
		importEdges := resolvePythonImportEdges(fg.lang, fg.elements, pythonImports, globalResolver, allByID)
		for _, e := range importEdges {
			key := e.Type + "|" + e.From + "|" + e.To
			if !seen[key] {
				seen[key] = true
				allEdges = append(allEdges, e)
			}
		}
		exportEdges := resolvePythonExportEdges(fg.lang, fg.elements, pythonExports, pythonImports, globalResolver, allByID)
		for _, e := range exportEdges {
			key := e.Type + "|" + e.From + "|" + e.To
			if !seen[key] {
				seen[key] = true
				allEdges = append(allEdges, e)
			}
		}

		result.Tree.Close()
	}

	return allEdges
}

// moduleNameFromElements returns the module FQName for file-scoped elements.
func moduleNameFromElements(elements []model.Element) string {
	for _, el := range elements {
		if el.Kind == model.KindModule {
			return el.FQName
		}
	}
	return ""
}

// resolvePythonImportEdges resolves internal Python IMPORTS edges from module imports.
func resolvePythonImportEdges(lang string, elements []model.Element, imports *pythonImportMaps, resolver *Resolver, allByID map[string]model.Element) []model.Edge {
	if lang != model.LangPython || imports == nil {
		return nil
	}
	moduleID := ""
	for _, el := range elements {
		if el.Kind == model.KindModule {
			moduleID = el.ID
			break
		}
	}
	if moduleID == "" {
		return nil
	}

	var edges []model.Edge
	for target := range imports.moduleTargets {
		toID, ok := resolver.Resolve(target)
		if !ok || toID == moduleID {
			continue
		}
		targetElem, hasTarget := allByID[toID]
		if !hasTarget || targetElem.Kind != model.KindModule {
			continue
		}
		edges = append(edges, model.Edge{From: moduleID, To: toID, Type: model.EdgeImports})
	}
	for target := range imports.symbolTargets {
		toID, ok := resolver.Resolve(target)
		if !ok || toID == moduleID {
			continue
		}
		edges = append(edges, model.Edge{From: moduleID, To: toID, Type: model.EdgeImports})
	}
	return edges
}

// resolvePythonExportEdges resolves __all__ exports to internal elements.
func resolvePythonExportEdges(lang string, elements []model.Element, exportNames []string, imports *pythonImportMaps, resolver *Resolver, allByID map[string]model.Element) []model.Edge {
	if lang != model.LangPython || len(exportNames) == 0 {
		return nil
	}
	moduleID := ""
	moduleName := ""
	for _, el := range elements {
		if el.Kind == model.KindModule {
			moduleID = el.ID
			moduleName = el.FQName
			break
		}
	}
	if moduleID == "" || moduleName == "" {
		return nil
	}

	var edges []model.Edge
	for _, name := range exportNames {
		if name == "" {
			continue
		}
		targetID, found := resolver.ResolveQualified(moduleName, name)
		if !found && imports != nil {
			if symbolFQName, ok := imports.symbolByName[name]; ok && symbolFQName != "" {
				targetID, found = resolver.Resolve(symbolFQName)
			}
		}
		if !found && imports != nil {
			if moduleFQName, ok := imports.qualifierByName[name]; ok && moduleFQName != "" {
				targetID, found = resolver.Resolve(moduleFQName)
			}
		}
		if !found || targetID == moduleID {
			continue
		}
		if _, ok := allByID[targetID]; !ok {
			continue
		}
		edges = append(edges, model.Edge{
			From: moduleID,
			To:   targetID,
			Type: model.EdgeExports,
		})
	}
	return edges
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

// enclosingKindsForLang returns the tree-sitter node kinds that represent functions for a language.
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

// isEnclosingKind checks if a node kind is in the list of enclosing kinds.
func isEnclosingKind(kind string, kinds []string) bool {
	for _, k := range kinds {
		if kind == k {
			return true
		}
	}
	return false
}
