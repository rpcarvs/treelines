package extractor

import (
	"fmt"
	"path/filepath"
	"strings"

	"lines/internal/model"
	"lines/internal/parser"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// RustExtractor extracts elements and edges from Rust source files.
type RustExtractor struct{}

// Extract parses Rust source and extracts elements and edges.
func (e *RustExtractor) Extract(result *parser.ParseResult) (*ExtractionResult, error) {
	queryStr, err := loadQuery(model.LangRust)
	if err != nil {
		return nil, err
	}

	tsLang := getLanguage(model.LangRust)
	root := result.Tree.RootNode()
	matches, captureNames, err := runQuery(queryStr, tsLang, root, result.Source)
	if err != nil {
		return nil, err
	}

	modulePath := rustModulePath(result.Path)
	var elements []model.Element
	var edges []model.Edge

	moduleID := model.MakeID(model.LangRust, result.Path, modulePath)
	moduleElem := model.Element{
		ID:         moduleID,
		Language:   model.LangRust,
		Kind:       model.KindModule,
		Name:       modulePath,
		FQName:     modulePath,
		Path:       result.Path,
		StartLine:  1,
		EndLine:    int(root.EndPosition().Row) + 1,
		LOC:        lineCount(root),
		Visibility: model.VisPublic,
	}
	elements = append(elements, moduleElem)

	implElements := make(map[string]model.Element)
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
		case "function_item":
			elem := rustFunctionElement(elementNode, name, modulePath, result)
			elements = append(elements, elem)
			elementsByNode[makeNodeKey(elementNode)] = elem.ID
			edges = append(edges, model.Edge{
				From: elem.ID,
				To:   moduleID,
				Type: model.EdgeDefinedIn,
			})
			if elem.Kind == model.KindMethod {
				parentImpl := findRustParentImpl(elementNode, implElements)
				if parentImpl != "" {
					edges = append(edges, model.Edge{
						From: parentImpl,
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

		case "struct_item":
			elem := rustTypeElement(elementNode, name, modulePath, model.KindStruct, result)
			elements = append(elements, elem)
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

		case "enum_item":
			elem := rustTypeElement(elementNode, name, modulePath, model.KindEnum, result)
			elements = append(elements, elem)
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

		case "trait_item":
			elem := rustTypeElement(elementNode, name, modulePath, model.KindTrait, result)
			elements = append(elements, elem)
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

		case "impl_item":
			traitNode := caps["trait_name"]
			elem := rustImplElement(elementNode, name, modulePath, traitNode, result)
			elements = append(elements, elem)
			implElements[elem.FQName] = elem
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
			if traitNode != nil {
				traitName := nodeText(traitNode, result.Source)
				traitFQ := modulePath + "::" + traitName
				traitID := model.MakeID(model.LangRust, result.Path, traitFQ)
				edges = append(edges, model.Edge{
					From: elem.ID,
					To:   traitID,
					Type: model.EdgeImplements,
				})
			}
		}
	}

	resolver := NewResolver(elements)
	elementsByID := buildElementsByID(elements)
	rustEnclosingKinds := []string{"function_item"}
	callEdges := extractCallEdges(matches, captureNames, result.Source, rustEnclosingKinds, elementsByNode, elementsByID, resolver, resolver)
	edges = append(edges, callEdges...)

	return &ExtractionResult{Elements: elements, Edges: edges}, nil
}

// rustFunctionElement builds an Element for a Rust function or method.
func rustFunctionElement(
	node *tree_sitter.Node,
	name, modulePath string,
	result *parser.ParseResult,
) model.Element {
	kind := model.KindFunction
	fqName := modulePath + "::" + name

	parent := node.Parent()
	if parent != nil && parent.Kind() == "declaration_list" {
		grandparent := parent.Parent()
		if grandparent != nil && grandparent.Kind() == "impl_item" {
			kind = model.KindMethod
			typeNode := grandparent.ChildByFieldName("type")
			if typeNode != nil {
				typeName := nodeText(typeNode, result.Source)
				fqName = modulePath + "::" + typeName + "::" + name
			}
		}
	}

	id := model.MakeID(model.LangRust, result.Path, fqName)
	return model.Element{
		ID:         id,
		Language:   model.LangRust,
		Kind:       kind,
		Name:       name,
		FQName:     fqName,
		Path:       result.Path,
		StartLine:  int(node.StartPosition().Row) + 1,
		EndLine:    int(node.EndPosition().Row) + 1,
		LOC:        lineCount(node),
		Signature:  signatureLine(node, result.Source),
		Visibility: rustVisibility(node),
		Docstring:  extractDocstring(node, result.Source, model.LangRust),
		Body:       nodeText(node, result.Source),
	}
}

// rustTypeElement builds an Element for a Rust struct, enum, or trait.
func rustTypeElement(
	node *tree_sitter.Node,
	name, modulePath, kind string,
	result *parser.ParseResult,
) model.Element {
	fqName := modulePath + "::" + name
	id := model.MakeID(model.LangRust, result.Path, fqName)
	return model.Element{
		ID:         id,
		Language:   model.LangRust,
		Kind:       kind,
		Name:       name,
		FQName:     fqName,
		Path:       result.Path,
		StartLine:  int(node.StartPosition().Row) + 1,
		EndLine:    int(node.EndPosition().Row) + 1,
		LOC:        lineCount(node),
		Signature:  signatureLine(node, result.Source),
		Visibility: rustVisibility(node),
		Docstring:  extractDocstring(node, result.Source, model.LangRust),
		Body:       nodeText(node, result.Source),
	}
}

// rustImplElement builds an Element for a Rust impl block.
func rustImplElement(
	node *tree_sitter.Node,
	name, modulePath string,
	traitNode *tree_sitter.Node,
	result *parser.ParseResult,
) model.Element {
	var fqName string
	if traitNode != nil {
		traitName := nodeText(traitNode, result.Source)
		fqName = modulePath + "::" + traitName + " for " + name
	} else {
		// Inherent impl blocks can appear multiple times for the same type in one file.
		// Include start line to keep each impl element ID unique and stable.
		fqName = fmt.Sprintf("%s::%s#impl@L%d", modulePath, name, int(node.StartPosition().Row)+1)
	}

	id := model.MakeID(model.LangRust, result.Path, fqName)
	return model.Element{
		ID:         id,
		Language:   model.LangRust,
		Kind:       model.KindImpl,
		Name:       name,
		FQName:     fqName,
		Path:       result.Path,
		StartLine:  int(node.StartPosition().Row) + 1,
		EndLine:    int(node.EndPosition().Row) + 1,
		LOC:        lineCount(node),
		Signature:  signatureLine(node, result.Source),
		Visibility: rustVisibility(node),
		Docstring:  extractDocstring(node, result.Source, model.LangRust),
		Body:       nodeText(node, result.Source),
	}
}

// findRustParentImpl finds the containing impl block ID for a method.
func findRustParentImpl(node *tree_sitter.Node, implElements map[string]model.Element) string {
	parent := node.Parent()
	if parent != nil && parent.Kind() == "declaration_list" {
		grandparent := parent.Parent()
		if grandparent != nil && grandparent.Kind() == "impl_item" {
			for _, elem := range implElements {
				if elem.StartLine == int(grandparent.StartPosition().Row)+1 {
					return elem.ID
				}
			}
		}
	}
	return ""
}

// rustVisibility returns visibility based on pub modifier presence.
func rustVisibility(node *tree_sitter.Node) string {
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child != nil && child.Kind() == "visibility_modifier" {
			return model.VisPublic
		}
	}
	return model.VisPrivate
}

// rustModulePath derives a crate-relative module path from a file path.
func rustModulePath(path string) string {
	path = filepath.ToSlash(path)

	if idx := strings.Index(path, "src/"); idx >= 0 {
		path = path[idx+4:]
	}

	path = strings.TrimSuffix(path, ".rs")

	if path == "lib" || path == "main" {
		return "crate"
	}

	path = strings.TrimSuffix(path, "/mod")
	parts := strings.Split(path, "/")
	return "crate::" + strings.Join(parts, "::")
}
