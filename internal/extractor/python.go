package extractor

import (
	"path/filepath"
	"strings"
	"unicode"

	"lines/internal/model"
	"lines/internal/parser"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// PythonExtractor extracts elements and edges from Python source files.
type PythonExtractor struct{}

func (e *PythonExtractor) Extract(result *parser.ParseResult) (*ExtractionResult, error) {
	queryStr, err := loadQuery(model.LangPython)
	if err != nil {
		return nil, err
	}

	tsLang := getLanguage(model.LangPython)
	root := result.Tree.RootNode()
	matches, captureNames, err := runQuery(queryStr, tsLang, root, result.Source)
	if err != nil {
		return nil, err
	}

	moduleName := pythonModuleName(result.Path)
	var elements []model.Element
	var edges []model.Edge

	moduleID := model.MakeID(model.LangPython, result.Path, moduleName)
	moduleElem := model.Element{
		ID:         moduleID,
		Language:   model.LangPython,
		Kind:       model.KindModule,
		Name:       moduleName,
		FQName:     moduleName,
		Path:       result.Path,
		StartLine:  1,
		EndLine:    int(root.EndPosition().Row) + 1,
		LOC:        lineCount(root),
		Visibility: model.VisPublic,
	}
	elements = append(elements, moduleElem)

	classElements := make(map[string]model.Element)
	elementsByNode := make(map[nodeKey]string)

	for _, m := range matches {
		caps := captureMap(m, captureNames)

		if _, hasImport := caps["import"]; hasImport {
			continue
		}

		elementNode, hasElement := caps["element"]
		nameNode, hasName := caps["name"]

		if !hasElement || !hasName {
			continue
		}

		name := nodeText(nameNode, result.Source)
		kind := elementNode.Kind()

		switch kind {
		case "function_definition":
			elem := pythonFunctionElement(elementNode, name, moduleName, result, classElements)
			elements = append(elements, elem)
			elementsByNode[makeNodeKey(elementNode)] = elem.ID
			edges = append(edges, model.Edge{
				From: elem.ID,
				To:   moduleID,
				Type: model.EdgeDefinedIn,
			})
			if elem.Kind == model.KindMethod {
				parentClass := findPythonParentClass(elementNode, classElements)
				if parentClass != "" {
					edges = append(edges, model.Edge{
						From: parentClass,
						To:   elem.ID,
						Type: model.EdgeContains,
					})
				}
			} else {
				edges = append(edges, model.Edge{
					From: moduleID,
					To:   elem.ID,
					Type: model.EdgeContains,
				})
			}

		case "class_definition":
			elem := pythonClassElement(elementNode, name, moduleName, result)
			elements = append(elements, elem)
			classElements[name] = elem
			edges = append(edges, model.Edge{
				From: elem.ID,
				To:   moduleID,
				Type: model.EdgeDefinedIn,
			})
			edges = append(edges, model.Edge{
				From: moduleID,
				To:   elem.ID,
				Type: model.EdgeContains,
			})
			basesNode := caps["bases"]
			if basesNode != nil {
				pythonExtractBases(basesNode, result, elem, moduleName, &edges)
			}
		}
	}

	resolver := NewResolver(elements)
	pyEnclosingKinds := []string{"function_definition"}
	callEdges := extractCallEdges(matches, captureNames, result.Source, pyEnclosingKinds, elementsByNode, resolver, resolver)
	edges = append(edges, callEdges...)

	return &ExtractionResult{Elements: elements, Edges: edges}, nil
}

func pythonFunctionElement(
	node *tree_sitter.Node,
	name, moduleName string,
	result *parser.ParseResult,
	classElements map[string]model.Element,
) model.Element {
	kind := model.KindFunction
	fqName := moduleName + "." + name

	parent := node.Parent()
	if parent != nil && parent.Kind() == "block" {
		grandparent := parent.Parent()
		if grandparent != nil && grandparent.Kind() == "class_definition" {
			kind = model.KindMethod
			classNameNode := grandparent.ChildByFieldName("name")
			if classNameNode != nil {
				className := nodeText(classNameNode, result.Source)
				fqName = moduleName + "." + className + "." + name
			}
		}
	}

	id := model.MakeID(model.LangPython, result.Path, fqName)
	return model.Element{
		ID:         id,
		Language:   model.LangPython,
		Kind:       kind,
		Name:       name,
		FQName:     fqName,
		Path:       result.Path,
		StartLine:  int(node.StartPosition().Row) + 1,
		EndLine:    int(node.EndPosition().Row) + 1,
		LOC:        lineCount(node),
		Signature:  signatureLine(node, result.Source),
		Visibility: pythonVisibility(name),
		Docstring:  extractDocstring(node, result.Source, model.LangPython),
		Body:       nodeText(node, result.Source),
	}
}

func pythonClassElement(
	node *tree_sitter.Node,
	name, moduleName string,
	result *parser.ParseResult,
) model.Element {
	fqName := moduleName + "." + name
	id := model.MakeID(model.LangPython, result.Path, fqName)
	return model.Element{
		ID:         id,
		Language:   model.LangPython,
		Kind:       model.KindClass,
		Name:       name,
		FQName:     fqName,
		Path:       result.Path,
		StartLine:  int(node.StartPosition().Row) + 1,
		EndLine:    int(node.EndPosition().Row) + 1,
		LOC:        lineCount(node),
		Signature:  signatureLine(node, result.Source),
		Visibility: pythonVisibility(name),
		Docstring:  extractDocstring(node, result.Source, model.LangPython),
		Body:       nodeText(node, result.Source),
	}
}

func pythonExtractBases(
	basesNode *tree_sitter.Node,
	result *parser.ParseResult,
	classElem model.Element,
	moduleName string,
	edges *[]model.Edge,
) {
	for i := uint(0); i < basesNode.ChildCount(); i++ {
		child := basesNode.Child(i)
		if child == nil {
			continue
		}
		kind := child.Kind()
		if kind == "identifier" || kind == "attribute" {
			baseName := nodeText(child, result.Source)
			baseID := model.MakeID(model.LangPython, result.Path, moduleName+"."+baseName)
			*edges = append(*edges, model.Edge{
				From: classElem.ID,
				To:   baseID,
				Type: model.EdgeExtends,
			})
		}
	}
}

func findPythonParentClass(node *tree_sitter.Node, classElements map[string]model.Element) string {
	parent := node.Parent()
	if parent != nil && parent.Kind() == "block" {
		grandparent := parent.Parent()
		if grandparent != nil && grandparent.Kind() == "class_definition" {
			classNameNode := grandparent.ChildByFieldName("name")
			if classNameNode != nil {
				className := classNameNode.Utf8Text(nil)
				if elem, ok := classElements[className]; ok {
					return elem.ID
				}
			}
		}
	}
	return ""
}

func pythonVisibility(name string) string {
	if len(name) > 0 && name[0] == '_' {
		return model.VisPrivate
	}
	return model.VisPublic
}


func pythonModuleName(path string) string {
	name := filepath.ToSlash(path)
	name = strings.TrimSuffix(name, ".py")
	name = strings.TrimSuffix(name, "/__init__")
	parts := strings.Split(name, "/")
	var filtered []string
	for _, p := range parts {
		if p == "" || p == "." {
			continue
		}
		filtered = append(filtered, p)
	}
	result := strings.Join(filtered, ".")
	runes := []rune(result)
	if len(runes) > 0 && !unicode.IsLetter(runes[0]) && runes[0] != '_' {
		return result
	}
	return result
}
