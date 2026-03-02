package extractor

import (
	"path/filepath"
	"strings"
	"unicode"
	"unicode/utf8"

	"lines/internal/model"
	"lines/internal/parser"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// GoExtractor extracts elements and edges from Go source files.
type GoExtractor struct{}

func (e *GoExtractor) Extract(result *parser.ParseResult) (*ExtractionResult, error) {
	queryStr, err := loadQuery(model.LangGo)
	if err != nil {
		return nil, err
	}

	tsLang := getLanguage(model.LangGo)
	root := result.Tree.RootNode()
	matches, captureNames, err := runQuery(queryStr, tsLang, root, result.Source)
	if err != nil {
		return nil, err
	}

	pkgName := goPackageName(root, result.Source)
	var elements []model.Element
	var edges []model.Edge

	dir := filepath.Dir(result.Path)
	moduleID := model.MakeID(model.LangGo, dir, pkgName)
	moduleElem := model.Element{
		ID:         moduleID,
		Language:   model.LangGo,
		Kind:       model.KindModule,
		Name:       pkgName,
		FQName:     pkgName,
		Path:       dir,
		StartLine:  0,
		EndLine:    0,
		LOC:        0,
		Visibility: model.VisPublic,
	}
	elements = append(elements, moduleElem)

	structElements := make(map[string]model.Element)
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
		case "function_declaration":
			elem := goFunctionElement(elementNode, name, pkgName, result)
			elements = append(elements, elem)
			elementsByNode[makeNodeKey(elementNode)] = elem.ID
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

		case "method_declaration":
			receiverNode := caps["receiver"]
			elem := goMethodElement(elementNode, name, pkgName, receiverNode, result)
			elements = append(elements, elem)
			elementsByNode[makeNodeKey(elementNode)] = elem.ID
			edges = append(edges, model.Edge{
				From: elem.ID,
				To:   moduleID,
				Type: model.EdgeDefinedIn,
			})
			receiverType := goReceiverType(receiverNode, result.Source)
			if receiverType != "" {
				if structElem, ok := structElements[receiverType]; ok {
					edges = append(edges, model.Edge{
						From: structElem.ID,
						To:   elem.ID,
						Type: model.EdgeContains,
					})
				}
			}

		case "type_declaration":
			typeKind, typeSpecKind := goTypeKind(elementNode, result.Source)
			if typeKind == "" {
				continue
			}
			fqName := pkgName + "." + name
			id := model.MakeID(model.LangGo, result.Path, fqName)
			elem := model.Element{
				ID:         id,
				Language:   model.LangGo,
				Kind:       typeKind,
				Name:       name,
				FQName:     fqName,
				Path:       result.Path,
				StartLine:  int(elementNode.StartPosition().Row) + 1,
				EndLine:    int(elementNode.EndPosition().Row) + 1,
				LOC:        lineCount(elementNode),
				Signature:  signatureLine(elementNode, result.Source),
				Visibility: goVisibility(name),
				Docstring:  extractDocstring(elementNode, result.Source, model.LangGo),
				Body:       nodeText(elementNode, result.Source),
			}
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
			if typeSpecKind == model.KindStruct {
				structElements[name] = elem
			}
		}
	}

	resolver := NewResolver(elements)
	goEnclosingKinds := []string{"function_declaration", "method_declaration"}
	callEdges := extractCallEdges(matches, captureNames, result.Source, goEnclosingKinds, elementsByNode, resolver, resolver)
	edges = append(edges, callEdges...)

	return &ExtractionResult{Elements: elements, Edges: edges}, nil
}

func goFunctionElement(
	node *tree_sitter.Node,
	name, pkgName string,
	result *parser.ParseResult,
) model.Element {
	fqName := pkgName + "." + name
	id := model.MakeID(model.LangGo, result.Path, fqName)
	return model.Element{
		ID:         id,
		Language:   model.LangGo,
		Kind:       model.KindFunction,
		Name:       name,
		FQName:     fqName,
		Path:       result.Path,
		StartLine:  int(node.StartPosition().Row) + 1,
		EndLine:    int(node.EndPosition().Row) + 1,
		LOC:        lineCount(node),
		Signature:  signatureLine(node, result.Source),
		Visibility: goVisibility(name),
		Docstring:  extractDocstring(node, result.Source, model.LangGo),
		Body:       nodeText(node, result.Source),
	}
}

func goMethodElement(
	node *tree_sitter.Node,
	name, pkgName string,
	receiverNode *tree_sitter.Node,
	result *parser.ParseResult,
) model.Element {
	receiverType := goReceiverType(receiverNode, result.Source)
	fqName := pkgName + "." + name
	if receiverType != "" {
		fqName = pkgName + "." + receiverType + "." + name
	}

	id := model.MakeID(model.LangGo, result.Path, fqName)
	return model.Element{
		ID:         id,
		Language:   model.LangGo,
		Kind:       model.KindMethod,
		Name:       name,
		FQName:     fqName,
		Path:       result.Path,
		StartLine:  int(node.StartPosition().Row) + 1,
		EndLine:    int(node.EndPosition().Row) + 1,
		LOC:        lineCount(node),
		Signature:  signatureLine(node, result.Source),
		Visibility: goVisibility(name),
		Docstring:  extractDocstring(node, result.Source, model.LangGo),
		Body:       nodeText(node, result.Source),
	}
}

func goReceiverType(receiverNode *tree_sitter.Node, source []byte) string {
	if receiverNode == nil {
		return ""
	}
	text := nodeText(receiverNode, source)
	text = strings.TrimPrefix(text, "(")
	text = strings.TrimSuffix(text, ")")
	parts := strings.Fields(text)
	if len(parts) == 0 {
		return ""
	}
	typePart := parts[len(parts)-1]
	typePart = strings.TrimPrefix(typePart, "*")
	return typePart
}

func goTypeKind(node *tree_sitter.Node, source []byte) (string, string) {
	text := nodeText(node, source)
	if strings.Contains(text, "struct {") || strings.Contains(text, "struct{") {
		return model.KindStruct, model.KindStruct
	}
	if strings.Contains(text, "interface {") || strings.Contains(text, "interface{") {
		return model.KindInterface, model.KindInterface
	}
	return "", ""
}

func goVisibility(name string) string {
	r, _ := utf8.DecodeRuneInString(name)
	if unicode.IsUpper(r) {
		return model.VisPublic
	}
	return model.VisPrivate
}


func goPackageName(root *tree_sitter.Node, source []byte) string {
	for i := uint(0); i < root.ChildCount(); i++ {
		child := root.Child(i)
		if child != nil && child.Kind() == "package_clause" {
			for j := uint(0); j < child.ChildCount(); j++ {
				sub := child.Child(j)
				if sub != nil && sub.Kind() == "package_identifier" {
					return nodeText(sub, source)
				}
			}
		}
	}
	return "main"
}
