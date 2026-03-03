package extractor

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"

	"lines/internal/model"
	"lines/internal/parser"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ResolveCrossPackageCalls re-parses indexed files and resolves
// cross-file edges against all known elements. It emits CALLS,
// IMPORTS, and Python EXPORTS edges.
func ResolveCrossPackageCalls(allElements []model.Element, p *parser.Parser, root string) []model.Edge {
	globalResolver := NewResolver(allElements)
	allByID := buildElementsByID(allElements)
	goModulesByDir := buildGoModulesByDir(allElements)
	goModulePrefix := readGoModulePrefix(root)

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
		var callImports *callImportMaps
		var pythonImports *pythonImportMaps
		var pythonExports []string
		if fg.lang == model.LangPython {
			pythonImports = extractPythonImportMaps(matches, captureNames, result.Source, moduleNameFromElements(fg.elements))
			pythonExports = extractPythonAllNames(matches, captureNames, result.Source)
			callImports = callImportMapsFromPython(pythonImports)
		}
		if fg.lang == model.LangRust {
			callImports = extractRustCallImportMaps(matches, captureNames, result.Source, moduleNameFromElements(fg.elements), globalResolver, allByID)
		}
		if fg.lang == model.LangGo {
			callImports = extractGoCallImportMaps(matches, captureNames, result.Source, goModulePrefix, goModulesByDir, allByID)
		}

		callEdges := extractCallEdges(matches, captureNames, result.Source, enclosingKinds, elementsByNode, elementsByID, localResolver, globalResolver, callImports)
		for _, e := range callEdges {
			key := e.From + "|" + e.To
			if !seen[key] {
				seen[key] = true
				allEdges = append(allEdges, e)
			}
		}
		importEdges := resolveImportEdges(
			fg.lang,
			fg.elements,
			matches,
			captureNames,
			result.Source,
			pythonImports,
			goModulePrefix,
			goModulesByDir,
			globalResolver,
			allByID,
		)
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

// resolveImportEdges resolves internal IMPORTS edges for supported languages.
func resolveImportEdges(
	lang string,
	elements []model.Element,
	matches []*tree_sitter.QueryMatch,
	captureNames []string,
	source []byte,
	pythonImports *pythonImportMaps,
	goModulePrefix string,
	goModulesByDir map[string]string,
	resolver *Resolver,
	allByID map[string]model.Element,
) []model.Edge {
	moduleID := moduleIDFromElements(elements)
	if moduleID == "" {
		return nil
	}

	switch lang {
	case model.LangPython:
		return resolvePythonImportEdges(moduleID, pythonImports, resolver, allByID)
	case model.LangGo:
		importPaths := parseGoImportPaths(matches, captureNames, source)
		return resolveGoImportEdges(moduleID, importPaths, goModulePrefix, goModulesByDir)
	case model.LangRust:
		usePaths := parseRustUsePaths(matches, captureNames, source)
		return resolveRustImportEdges(moduleID, usePaths, resolver, allByID)
	default:
		return nil
	}
}

// resolvePythonImportEdges resolves internal Python IMPORTS edges from module imports.
func resolvePythonImportEdges(moduleID string, imports *pythonImportMaps, resolver *Resolver, allByID map[string]model.Element) []model.Edge {
	if imports == nil {
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

// resolveGoImportEdges resolves internal Go import paths to module elements.
func resolveGoImportEdges(moduleID string, imports []string, modulePrefix string, modulesByDir map[string]string) []model.Edge {
	var edges []model.Edge
	seen := make(map[string]struct{})
	for _, imp := range imports {
		targetDir := goImportToDir(imp, modulePrefix)
		if targetDir == "" {
			continue
		}
		targetID, ok := modulesByDir[targetDir]
		if !ok || targetID == moduleID {
			continue
		}
		key := moduleID + "|" + targetID
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		edges = append(edges, model.Edge{From: moduleID, To: targetID, Type: model.EdgeImports})
	}
	return edges
}

// resolveRustImportEdges resolves internal Rust use paths to module/symbol elements.
func resolveRustImportEdges(moduleID string, usePaths []string, resolver *Resolver, allByID map[string]model.Element) []model.Edge {
	var edges []model.Edge
	seen := make(map[string]struct{})
	for _, usePath := range usePaths {
		if !strings.HasPrefix(usePath, "crate::") {
			continue
		}
		targetID, ok := resolveRustImportTarget(usePath, resolver, allByID)
		if !ok || targetID == moduleID {
			continue
		}
		key := moduleID + "|" + targetID
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		edges = append(edges, model.Edge{From: moduleID, To: targetID, Type: model.EdgeImports})
	}
	return edges
}

// resolveRustImportTarget resolves a crate:: path to an internal element ID.
func resolveRustImportTarget(path string, resolver *Resolver, allByID map[string]model.Element) (string, bool) {
	if id, ok := resolver.Resolve(path); ok {
		return id, true
	}
	parts := strings.Split(path, "::")
	for i := len(parts) - 1; i >= 2; i-- {
		candidate := strings.Join(parts[:i], "::")
		id, ok := resolver.Resolve(candidate)
		if !ok {
			continue
		}
		if elem, exists := allByID[id]; exists && elem.Kind == model.KindModule {
			return id, true
		}
	}
	return "", false
}

// moduleIDFromElements returns module element ID for a file group.
func moduleIDFromElements(elements []model.Element) string {
	for _, el := range elements {
		if el.Kind == model.KindModule {
			return el.ID
		}
	}
	return ""
}

// buildGoModulesByDir maps relative Go package dirs to module IDs.
func buildGoModulesByDir(allElements []model.Element) map[string]string {
	modules := make(map[string]string)
	for _, el := range allElements {
		if el.Language != model.LangGo || el.Kind != model.KindModule {
			continue
		}
		dir := filepath.Clean(filepath.ToSlash(el.Path))
		modules[dir] = el.ID
	}
	return modules
}

// readGoModulePrefix reads go.mod module prefix for internal import mapping.
func readGoModulePrefix(root string) string {
	data, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		return ""
	}
	lines := strings.Split(string(data), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if !strings.HasPrefix(line, "module ") {
			continue
		}
		return strings.TrimSpace(strings.TrimPrefix(line, "module "))
	}
	return ""
}

// goImportToDir converts a Go import path into a relative package directory.
func goImportToDir(importPath, modulePrefix string) string {
	importPath = filepath.ToSlash(strings.TrimSpace(importPath))
	if importPath == "" {
		return ""
	}
	if modulePrefix != "" {
		prefix := modulePrefix + "/"
		if strings.HasPrefix(importPath, prefix) {
			return filepath.Clean(strings.TrimPrefix(importPath, prefix))
		}
	}
	return ""
}

// extractGoCallImportMaps resolves Go import bindings to package qualifiers.
func extractGoCallImportMaps(matches []*tree_sitter.QueryMatch, captureNames []string, source []byte, modulePrefix string, modulesByDir map[string]string, allByID map[string]model.Element) *callImportMaps {
	specs := parseGoImportSpecs(matches, captureNames, source)
	if len(specs) == 0 {
		return nil
	}
	maps := newCallImportMaps()
	for _, spec := range specs {
		dir := goImportToDir(spec.Path, modulePrefix)
		if dir == "" {
			continue
		}
		moduleID, ok := modulesByDir[dir]
		if !ok {
			continue
		}
		moduleElem, ok := allByID[moduleID]
		if !ok || moduleElem.Kind != model.KindModule {
			continue
		}
		if spec.Binding == "" {
			continue
		}
		maps.qualifierByName[spec.Binding] = moduleElem.FQName
	}
	if len(maps.qualifierByName) == 0 {
		return nil
	}
	return maps
}

var rustModDeclRe = regexp.MustCompile(`(?m)^\s*mod\s+([A-Za-z_]\w*)\s*;`)

// extractRustCallImportMaps resolves Rust call qualifiers and imported symbol aliases.
func extractRustCallImportMaps(matches []*tree_sitter.QueryMatch, captureNames []string, source []byte, moduleName string, resolver *Resolver, allByID map[string]model.Element) *callImportMaps {
	maps := newCallImportMaps()
	for _, b := range parseRustUseBindings(matches, captureNames, source) {
		if b.Target == "" || b.Alias == "" || b.Alias == "_" {
			continue
		}
		maps.qualifierByName[b.Alias] = b.Target
		id, ok := resolver.Resolve(b.Target)
		if !ok {
			continue
		}
		target, ok := allByID[id]
		if !ok || target.Kind == model.KindModule {
			continue
		}
		maps.symbolByName[b.Alias] = b.Target
	}

	if moduleName != "" {
		for _, m := range rustModDeclRe.FindAllSubmatch(source, -1) {
			if len(m) < 2 {
				continue
			}
			modName := string(m[1])
			if modName == "" {
				continue
			}
			maps.qualifierByName[modName] = moduleName + "::" + modName
		}
	}

	if len(maps.qualifierByName) == 0 && len(maps.symbolByName) == 0 {
		return nil
	}
	return maps
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
