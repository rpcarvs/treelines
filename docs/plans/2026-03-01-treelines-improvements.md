# Treelines Improvements Implementation Plan

> **For Claude:** REQUIRED SUB-SKILL: Use superpowers:executing-plans to implement this plan task-by-task.

**Goal:** Make treelines useful for agents by storing source bodies, fixing FQName accuracy, enabling smarter lookups, and resolving cross-package calls.

**Architecture:** Five independent improvements to the existing extraction/storage pipeline. Tasks 1-2 are foundational (schema + model changes), Task 3 modifies the element command, Task 4 adds a post-index resolution pass, Task 5 renames the DB file.

**Tech Stack:** Go 1.25.0, SQLite via modernc.org/sqlite, tree-sitter

---

### Task 1: Add body column to elements

**Files:**
- Modify: `internal/model/element.go:31-44`
- Modify: `internal/graph/schema.go:6-18`
- Modify: `internal/graph/sqlitestore.go:46-62` (UpsertElement)
- Modify: `internal/graph/sqlitestore.go:196-216` (queryElements)
- Modify: `internal/graph/sqlitestore.go:218-225` (scanElement)

**Step 1: Add Body field to Element struct**

In `internal/model/element.go`, add `Body` field after `Docstring`:

```go
type Element struct {
	ID         string `json:"id"`
	Language   string `json:"language"`
	Kind       string `json:"kind"`
	Name       string `json:"name"`
	FQName     string `json:"fq_name"`
	Path       string `json:"path"`
	StartLine  int    `json:"start_line"`
	EndLine    int    `json:"end_line"`
	LOC        int    `json:"loc"`
	Signature  string `json:"signature"`
	Visibility string `json:"visibility"`
	Docstring  string `json:"docstring"`
	Body       string `json:"body"`
}
```

**Step 2: Add body column to schema DDL**

In `internal/graph/schema.go`, add `body TEXT NOT NULL DEFAULT ''` after `docstring`:

```go
`CREATE TABLE IF NOT EXISTS elements (
    id TEXT PRIMARY KEY,
    language TEXT NOT NULL,
    kind TEXT NOT NULL,
    name TEXT NOT NULL,
    fq_name TEXT NOT NULL,
    path TEXT NOT NULL,
    start_line INTEGER NOT NULL,
    end_line INTEGER NOT NULL,
    loc INTEGER NOT NULL,
    signature TEXT NOT NULL DEFAULT '',
    visibility TEXT NOT NULL DEFAULT '',
    docstring TEXT NOT NULL DEFAULT '',
    body TEXT NOT NULL DEFAULT ''
)`,
```

**Step 3: Update UpsertElement to include body**

In `internal/graph/sqlitestore.go`, update `UpsertElement`:

```go
func (s *SQLiteStore) UpsertElement(el model.Element) error {
	query := `INSERT INTO elements (id, language, kind, name, fq_name, path, start_line, end_line, loc, signature, visibility, docstring, body)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(id) DO UPDATE SET
			language=excluded.language, kind=excluded.kind, name=excluded.name,
			fq_name=excluded.fq_name, path=excluded.path, start_line=excluded.start_line,
			end_line=excluded.end_line, loc=excluded.loc, signature=excluded.signature,
			visibility=excluded.visibility, docstring=excluded.docstring, body=excluded.body`
	_, err := s.db.Exec(query,
		el.ID, el.Language, el.Kind, el.Name, el.FQName, el.Path,
		el.StartLine, el.EndLine, el.LOC, el.Signature, el.Visibility, el.Docstring, el.Body,
	)
	if err != nil {
		return fmt.Errorf("upsert element: %w", err)
	}
	return nil
}
```

**Step 4: Update queryElements and scanElement to scan body**

In `internal/graph/sqlitestore.go`, update both scan calls to include `&el.Body`:

`queryElements`:
```go
err := rows.Scan(
    &el.ID, &el.Language, &el.Kind, &el.Name, &el.FQName, &el.Path,
    &el.StartLine, &el.EndLine, &el.LOC, &el.Signature, &el.Visibility, &el.Docstring, &el.Body,
)
```

`scanElement`:
```go
err := row.Scan(
    &el.ID, &el.Language, &el.Kind, &el.Name, &el.FQName, &el.Path,
    &el.StartLine, &el.EndLine, &el.LOC, &el.Signature, &el.Visibility, &el.Docstring, &el.Body,
)
```

**Step 5: Populate Body in all three extractors**

In each extractor, after creating an Element, set `Body: nodeText(elementNode, result.Source)`.

For `internal/extractor/golang.go`, update `goFunctionElement`, `goMethodElement`, and the type_declaration case inside `Extract` to set Body. Example for `goFunctionElement`:

```go
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
```

Apply the same pattern to:
- `golang.go`: `goMethodElement`, type_declaration case in `Extract`
- `python.go`: `pythonFunctionElement`, `pythonClassElement`
- `rust.go`: `rustFunctionElement`, `rustTypeElement`, `rustImplElement`

Also set `Body` on module elements in each extractor's `Extract` method (set to empty string, modules don't need body).

**Step 6: Verify**

Run: `go build .`
Expected: Compiles without errors.

Run: `go vet ./...`
Expected: No issues.

---

### Task 2: Fix FQName generation

**Files:**
- Modify: `internal/extractor/golang.go:30,112,158,183-186`
- Modify: `internal/extractor/python.go:30,133,170`

**Context:** Go extractor uses `goPackageName()` which correctly reads the `package` clause (e.g. "graph", "cmd", "model"). The issue identified during assessment was that relative paths were being used. The Go extractor already does this correctly via `goPackageName`. However, the Python extractor derives module name from the full file path, which can include leading path components that aren't meaningful.

**Step 1: Fix Python module name derivation**

In `internal/extractor/python.go`, update `pythonModuleName` to strip the root path prefix and produce cleaner names. The function already handles this, but for files like `internal/extractor/python.go` it produces `internal.extractor.python`. For Python, the convention is to use the file path relative to root, so this is correct. No change needed for Python.

**Step 2: Verify Go FQName correctness**

The Go extractor already uses `goPackageName()` which reads the actual `package` declaration. For `internal/graph/sqlitestore.go`, it reads `package graph` and produces FQNames like `graph.SQLiteStore.Open`. This is correct.

Re-index treelines and verify:

Run: `rm -rf .treelines && go run . init && go run . index --verbose`

Run: `go run . element "graph.SQLiteStore"`
Expected: Returns the SQLiteStore element with FQName `graph.SQLiteStore`, not `main.SQLiteStore`.

Run: `go run . element "cmd.runIndex"`
Expected: Returns with FQName `cmd.runIndex`.

**Note:** If the previous assessment showed `main.*` for all elements, the bug was that the test was done on the treelines binary itself where `main.go` is in package `main`. Packages like `graph`, `cmd`, `extractor` should already have correct FQNames. Verify this first, and only make changes if FQNames are actually wrong.

---

### Task 3: Smarter element lookup

**Files:**
- Modify: `internal/graph/store.go:16`
- Modify: `internal/graph/sqlitestore.go:102-117`
- Modify: `cmd/element.go:20-51`

**Step 1: Add GetElementByExactName method to GraphStore interface**

In `internal/graph/store.go`, add between `GetElement` and `GetElementsByName`:

```go
GetElementByExactName(name string) ([]model.Element, error)
```

**Step 2: Implement GetElementByExactName in SQLiteStore**

In `internal/graph/sqlitestore.go`, add after `GetElement`:

```go
func (s *SQLiteStore) GetElementByExactName(name string) ([]model.Element, error) {
	query := `SELECT * FROM elements WHERE name = ?`
	return s.queryElements(query, name)
}
```

**Step 3: Update element command with three-step lookup**

In `cmd/element.go`, replace `runElement`:

```go
func runElement(cmd *cobra.Command, args []string) error {
	root, err := resolveRoot()
	if err != nil {
		return err
	}

	store, err := openStore(root)
	if err != nil {
		return err
	}
	defer func() { _ = store.Close() }()

	name := args[0]

	// Step 1: Exact FQName match
	el, err := store.GetElement(name)
	if err != nil {
		return fmt.Errorf("get element: %w", err)
	}
	if el != nil {
		return output(el)
	}

	// Step 2: Exact name match
	exact, err := store.GetElementByExactName(name)
	if err != nil {
		return fmt.Errorf("exact name search: %w", err)
	}
	if len(exact) > 0 {
		return output(exact)
	}

	// Step 3: Substring search
	elements, err := store.GetElementsByName(name)
	if err != nil {
		return fmt.Errorf("search elements: %w", err)
	}
	if len(elements) == 0 {
		return fmt.Errorf("no element found matching %q", name)
	}

	return output(elements)
}
```

**Step 4: Verify**

Run: `go build .`
Expected: Compiles without errors.

Run: `rm -rf .treelines && go run . init && go run . index`

Run: `go run . element "Open"`
Expected: Returns all elements named exactly "Open" (e.g., `graph.SQLiteStore.Open`).

Run: `go run . element "graph.SQLiteStore.Open"`
Expected: Returns single element via FQName match.

Run: `go run . element "Upsert"`
Expected: Returns elements via substring match (UpsertElement, UpsertEdge).

---

### Task 4: Post-index cross-package call resolution

**Files:**
- Create: `internal/extractor/crossref.go`
- Modify: `internal/graph/store.go` (add GetAllElements, DeleteEdgesByType)
- Modify: `internal/graph/sqlitestore.go` (implement new methods)
- Modify: `cmd/index.go:25-94`
- Modify: `cmd/update.go:26-126`

**Step 1: Add GetAllElements and DeleteEdgesByType to GraphStore**

In `internal/graph/store.go`, add:

```go
GetAllElements() ([]model.Element, error)
DeleteEdgesByType(edgeType string) error
```

**Step 2: Implement in SQLiteStore**

In `internal/graph/sqlitestore.go`, add:

```go
func (s *SQLiteStore) GetAllElements() ([]model.Element, error) {
	return s.queryElements(`SELECT * FROM elements`)
}

func (s *SQLiteStore) DeleteEdgesByType(edgeType string) error {
	_, err := s.db.Exec(`DELETE FROM edges WHERE type = ?`, edgeType)
	if err != nil {
		return fmt.Errorf("delete edges by type: %w", err)
	}
	return nil
}
```

**Step 3: Create crossref.go**

Create `internal/extractor/crossref.go`:

```go
package extractor

import (
	"lines/internal/model"
	"lines/internal/parser"

	tree_sitter "github.com/tree-sitter/go-tree-sitter"
)

// ResolveCrossPackageCalls re-parses all indexed files and resolves call
// expressions against the full element database. Returns CALLS edges
// with cross-package visibility.
func ResolveCrossPackageCalls(
	allElements []model.Element,
	p *parser.Parser,
) []model.Edge {
	resolver := NewResolver(allElements)

	elementsByFile := make(map[string][]model.Element)
	for _, el := range allElements {
		elementsByFile[el.Path] = append(elementsByFile[el.Path], el)
	}

	var edges []model.Edge
	seen := make(map[string]bool)

	for path, fileElements := range elementsByFile {
		if len(fileElements) == 0 {
			continue
		}
		lang := fileElements[0].Language

		result, err := p.ParseFile(path, lang)
		if err != nil {
			continue
		}
		defer result.Tree.Close()

		queryStr, err := loadQuery(lang)
		if err != nil {
			continue
		}

		tsLang := getLanguage(lang)
		root := result.Tree.RootNode()
		matches, captureNames, err := runQuery(queryStr, tsLang, root, result.Source)
		if err != nil {
			continue
		}

		elementsByNode := make(map[nodeKey]string)
		for _, el := range fileElements {
			if el.Kind == model.KindModule {
				continue
			}
			startByte := findNodeStartByte(root, el.StartLine)
			if startByte >= 0 {
				elementsByNode[nodeKey{startByte: uint(startByte), endByte: 0}] = el.ID
			}
		}

		nodeMap := buildNodeMap(matches, captureNames, result.Source, lang)
		for nodeID, callerID := range nodeMap {
			elementsByNode[nodeID] = callerID
		}

		enclosingKinds := enclosingKindsForLang(lang)
		callEdges := extractCallEdges(matches, captureNames, result.Source, enclosingKinds, elementsByNode, resolver)
		for _, e := range callEdges {
			key := e.From + "->" + e.To
			if !seen[key] {
				seen[key] = true
				edges = append(edges, e)
			}
		}
	}

	return edges
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

func buildNodeMap(
	matches []*tree_sitter.QueryMatch,
	captureNames []string,
	source []byte,
	lang string,
) map[nodeKey]string {
	result := make(map[nodeKey]string)
	for _, m := range matches {
		caps := captureMap(m, captureNames)
		elementNode, hasElement := caps["element"]
		nameNode, hasName := caps["name"]
		if !hasElement || !hasName {
			continue
		}
		kind := elementNode.Kind()
		name := nodeText(nameNode, source)
		_ = name
		switch lang {
		case model.LangGo:
			if kind == "function_declaration" || kind == "method_declaration" {
				result[makeNodeKey(elementNode)] = ""
			}
		case model.LangPython:
			if kind == "function_definition" {
				result[makeNodeKey(elementNode)] = ""
			}
		case model.LangRust:
			if kind == "function_item" {
				result[makeNodeKey(elementNode)] = ""
			}
		}
	}
	return result
}

func findNodeStartByte(root *tree_sitter.Node, startLine int) int {
	return findNodeAtLine(root, uint(startLine-1))
}

func findNodeAtLine(node *tree_sitter.Node, line uint) int {
	if node.StartPosition().Row == line {
		return int(node.StartByte())
	}
	for i := uint(0); i < node.ChildCount(); i++ {
		child := node.Child(i)
		if child == nil {
			continue
		}
		if child.StartPosition().Row <= line && child.EndPosition().Row >= line {
			result := findNodeAtLine(child, line)
			if result >= 0 {
				return result
			}
		}
	}
	return -1
}
```

Wait, this approach is getting complex. Let me simplify. The cross-ref pass just needs to re-use the existing extraction with a global resolver instead of the per-file resolver. A simpler approach:

Replace `crossref.go` with:

```go
package extractor

import (
	"lines/internal/model"
	"lines/internal/parser"
)

// ResolveCrossPackageCalls re-parses all indexed files and resolves call
// expressions against the full set of known elements. Returns new CALLS
// edges that span package boundaries.
func ResolveCrossPackageCalls(
	allElements []model.Element,
	p *parser.Parser,
) []model.Edge {
	globalResolver := NewResolver(allElements)

	type fileInfo struct {
		path string
		lang string
	}
	files := make(map[string]fileInfo)
	elementsByNode := make(map[string]map[nodeKey]string)

	for _, el := range allElements {
		if el.Kind == model.KindModule || el.Path == "external" {
			continue
		}
		fi := fileInfo{path: el.Path, lang: el.Language}
		files[el.Path] = fi
	}

	var edges []model.Edge
	seen := make(map[string]bool)

	for _, fi := range files {
		result, err := p.ParseFile(fi.path, fi.lang)
		if err != nil {
			continue
		}

		queryStr, err := loadQuery(fi.lang)
		if err != nil {
			result.Tree.Close()
			continue
		}

		tsLang := getLanguage(fi.lang)
		root := result.Tree.RootNode()
		matches, captureNames, err := runQuery(queryStr, tsLang, root, result.Source)
		if err != nil {
			result.Tree.Close()
			continue
		}

		nodeMap := make(map[nodeKey]string)
		enclosingKinds := enclosingKindsForLang(fi.lang)

		for _, m := range matches {
			caps := captureMap(m, captureNames)
			elementNode, hasElement := caps["element"]
			nameNode, hasName := caps["name"]
			if !hasElement || !hasName {
				continue
			}
			name := nodeText(nameNode, result.Source)
			kind := elementNode.Kind()
			if isEnclosingKind(kind, enclosingKinds) {
				id, found := globalResolver.Resolve(name)
				if !found {
					pkgPrefix := guessPkgPrefix(fi.lang, result, root)
					fqName := pkgPrefix + langSep(fi.lang) + name
					id, found = globalResolver.ResolveFQName(fqName)
				}
				if found {
					nodeMap[makeNodeKey(elementNode)] = id
				}
			}
		}

		callEdges := extractCallEdges(matches, captureNames, result.Source, enclosingKinds, nodeMap, globalResolver)
		for _, e := range callEdges {
			key := e.From + "|" + e.To
			if !seen[key] {
				seen[key] = true
				edges = append(edges, e)
			}
		}

		result.Tree.Close()
	}

	return edges
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

func langSep(lang string) string {
	if lang == model.LangRust {
		return "::"
	}
	return "."
}
```

This is still getting complicated due to needing to map tree-sitter nodes back to element IDs. Let me take the simplest approach: since the extractors already build `elementsByNode` maps and call `extractCallEdges`, the cross-ref pass just needs to run the same extraction logic but with a global resolver. The cleanest way is to:

1. Add a `Resolver` parameter to extractors (or a global elements list)
2. After initial indexing, fetch all elements, build a global resolver, and re-run extraction with it

But that requires changing the Extractor interface. A simpler approach: just re-extract each file and build a fresh `elementsByNode` map from the matches (the same way extractors do), then call `extractCallEdges` with the global resolver.

Let me write a cleaner version of crossref.go and the integration.

**Actually, let me simplify the plan significantly.** The crossref step should:
1. Get all elements from DB
2. Build a global Resolver
3. For each file, re-parse and re-run queries
4. For each file, build elementsByNode from the query matches (same logic each extractor uses)
5. Call extractCallEdges with the global resolver
6. Insert any new CALLS edges

Create `internal/extractor/crossref.go`:

```go
package extractor

import (
	"lines/internal/model"
	"lines/internal/parser"
)

// ResolveCrossPackageCalls re-parses indexed files and resolves call
// expressions against all known elements, producing CALLS edges that
// span package boundaries.
func ResolveCrossPackageCalls(allElements []model.Element, p *parser.Parser) []model.Edge {
	globalResolver := NewResolver(allElements)

	type fileGroup struct {
		lang     string
		elements []model.Element
	}
	byFile := make(map[string]*fileGroup)
	for _, el := range allElements {
		if el.Path == "external" {
			continue
		}
		fg, ok := byFile[el.Path]
		if !ok {
			fg = &fileGroup{lang: el.Language}
			byFile[el.Path] = fg
		}
		fg.elements = append(fg.elements, el)
	}

	var allEdges []model.Edge
	seen := make(map[string]bool)

	for path, fg := range byFile {
		result, err := p.ParseFile(path, fg.lang)
		if err != nil {
			continue
		}

		queryStr, err := loadQuery(fg.lang)
		if err != nil {
			result.Tree.Close()
			continue
		}

		tsLang := getLanguage(fg.lang)
		root := result.Tree.RootNode()
		matches, captureNames, err := runQuery(queryStr, tsLang, root, result.Source)
		if err != nil {
			result.Tree.Close()
			continue
		}

		enclosingKinds := enclosingKindsForLang(fg.lang)
		elementsByNode := mapElementsToNodes(matches, captureNames, result.Source, fg.elements, enclosingKinds)

		callEdges := extractCallEdges(matches, captureNames, result.Source, enclosingKinds, elementsByNode, globalResolver)
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
// nodes by comparing the element name with @name captures in query matches.
func mapElementsToNodes(
	matches []*tree_sitter.QueryMatch,
	captureNames []string,
	source []byte,
	fileElements []model.Element,
	enclosingKinds []string,
) map[nodeKey]string {
	// Build a lookup from (name, startLine) to element ID
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
```

Need to add the missing import. Actually, `mapElementsToNodes` uses `tree_sitter.QueryMatch` so needs the import.

**Step 4: Add ResolveFQName to Resolver**

In `internal/extractor/resolver.go`, add:

```go
func (r *Resolver) ResolveFQName(fqName string) (string, bool) {
	id, ok := r.fqNames[fqName]
	return id, ok
}
```

**Step 5: Wire cross-package resolution into index command**

In `cmd/index.go`, after the main extraction loop and before saving the commit hash, add:

```go
	logInfo("Resolving cross-package calls...")
	allElements, err := store.GetAllElements()
	if err != nil {
		logVerbose("Get all elements for cross-ref: %v", err)
	} else {
		if err := store.DeleteEdgesByType(model.EdgeCalls); err != nil {
			logVerbose("Delete old CALLS edges: %v", err)
		}
		crossEdges := extractor.ResolveCrossPackageCalls(allElements, p)
		for _, e := range crossEdges {
			if err := store.UpsertEdge(e); err != nil {
				logVerbose("Upsert cross-ref edge: %v", err)
			}
		}
		logInfo("Resolved %d cross-package call edges", len(crossEdges))
	}
```

**Step 6: Wire cross-package resolution into update command**

In `cmd/update.go`, add the same cross-ref block after the file loop, before saving the commit hash (line 119).

**Step 7: Verify**

Run: `go build .`
Expected: Compiles without errors.

Run: `rm -rf .treelines && go run . init && go run . index --verbose`
Expected: See "Resolving cross-package calls..." and a count of resolved edges.

Run: `go run . uses "graph.SQLiteStore.Open"`
Expected: Shows callers from cmd package (e.g., `cmd.openStore` or `cmd.runInit`).

---

### Task 5: Change database filename

**Files:**
- Modify: `cmd/helpers.go:26`

**Step 1: Update dbPath default**

In `cmd/helpers.go`, change the default path:

```go
func dbPath(root string) string {
	if flagDB != "" {
		return flagDB
	}
	return filepath.Join(root, ".treelines", "codestore.db")
}
```

**Step 2: Verify**

Run: `go build .`
Expected: Compiles.

Run: `rm -rf .treelines && go run . init && ls .treelines/`
Expected: Shows `codestore.db` file.

---

## Verification

After all tasks are complete, run the full integration test:

```bash
go build .
go vet ./...
rm -rf .treelines
./lines init
./lines index --verbose
./lines element "graph.SQLiteStore"
./lines element "Open"
./lines element "Upsert"
./lines uses "graph.SQLiteStore.Open"
./lines search "Extract"
```

Check that:
1. `lines element "graph.SQLiteStore"` returns the struct with a `body` field containing the full source
2. `lines element "Open"` returns exact name matches (not substring)
3. `lines element "Upsert"` returns substring matches for UpsertElement/UpsertEdge
4. `lines uses "graph.SQLiteStore.Open"` shows cross-package callers
5. The DB file is `.treelines/codestore.db`
