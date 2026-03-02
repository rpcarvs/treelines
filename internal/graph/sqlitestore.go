package graph

import (
	"database/sql"
	"fmt"

	"lines/internal/model"

	_ "modernc.org/sqlite"
)

// SQLiteStore provides SQLite-backed storage for elements and edges.
type SQLiteStore struct {
	db *sql.DB
}

// NewSQLiteStore creates a new uninitialized SQLiteStore.
func NewSQLiteStore() *SQLiteStore {
	return &SQLiteStore{}
}

// Open opens a SQLite database at the given path.
func (s *SQLiteStore) Open(path string) error {
	db, err := sql.Open("sqlite", path+"?_busy_timeout=5000")
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	s.db = db
	return nil
}

// Close closes the database connection.
func (s *SQLiteStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

// CreateSchema creates the elements and edges tables if they don't exist.
func (s *SQLiteStore) CreateSchema() error {
	for _, stmt := range SchemaStatements() {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("create schema: %w", err)
		}
	}
	return nil
}

// Reset clears all graph data so a full index can rebuild an authoritative snapshot.
func (s *SQLiteStore) Reset() error {
	tx, err := s.db.Begin()
	if err != nil {
		return fmt.Errorf("begin reset transaction: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.Exec(`DELETE FROM edges`); err != nil {
		return fmt.Errorf("delete edges: %w", err)
	}
	if _, err := tx.Exec(`DELETE FROM elements`); err != nil {
		return fmt.Errorf("delete elements: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit reset transaction: %w", err)
	}
	return nil
}

// UpsertElement inserts or updates an element in the database.
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

// UpsertEdge inserts an edge or ignores it if it already exists.
func (s *SQLiteStore) UpsertEdge(e model.Edge) error {
	query := `INSERT INTO edges (from_id, to_id, type) VALUES (?, ?, ?)
		ON CONFLICT(from_id, to_id, type) DO NOTHING`
	_, err := s.db.Exec(query, e.From, e.To, e.Type)
	if err != nil {
		return fmt.Errorf("upsert edge: %w", err)
	}
	return nil
}

// DeleteEdgesForFile removes all edges referencing elements in a file.
func (s *SQLiteStore) DeleteEdgesForFile(path string) error {
	query := `DELETE FROM edges WHERE
		from_id IN (SELECT id FROM elements WHERE path = ?) OR
		to_id IN (SELECT id FROM elements WHERE path = ?)`
	_, err := s.db.Exec(query, path, path)
	if err != nil {
		return fmt.Errorf("delete edges for file: %w", err)
	}
	return nil
}

// DeleteElementsByFile removes all elements belonging to the given file path.
func (s *SQLiteStore) DeleteElementsByFile(path string) error {
	_, err := s.db.Exec(`DELETE FROM elements WHERE path = ?`, path)
	if err != nil {
		return fmt.Errorf("delete elements by file: %w", err)
	}
	return nil
}

// GetElement retrieves an element by its fully qualified name.
func (s *SQLiteStore) GetElement(fqName string) (*model.Element, error) {
	row := s.db.QueryRow(`SELECT * FROM elements WHERE fq_name = ? LIMIT 1`, fqName)
	el, err := scanElement(row)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("get element: %w", err)
	}
	return &el, nil
}

// GetElementByExactName retrieves elements matching an exact short name.
func (s *SQLiteStore) GetElementByExactName(name string) ([]model.Element, error) {
	query := `SELECT * FROM elements WHERE name = ?`
	return s.queryElements(query, name)
}

// GetElementsByName retrieves elements whose name contains a substring.
func (s *SQLiteStore) GetElementsByName(name string) ([]model.Element, error) {
	query := `SELECT * FROM elements WHERE name LIKE '%' || ? || '%'`
	return s.queryElements(query, name)
}

// GetCallers returns elements that call the given element.
func (s *SQLiteStore) GetCallers(fqName string) ([]model.Element, error) {
	query := `SELECT DISTINCT e.* FROM elements e
		JOIN edges ed ON ed.from_id = e.id
		JOIN elements t ON ed.to_id = t.id
		WHERE ed.type = ? AND t.fq_name = ?`
	return s.queryElements(query, model.EdgeCalls, fqName)
}

// GetCallees returns elements called by the given element.
func (s *SQLiteStore) GetCallees(fqName string) ([]model.Element, error) {
	query := `SELECT DISTINCT e.* FROM elements e
		JOIN edges ed ON ed.to_id = e.id
		JOIN elements src ON ed.from_id = src.id
		WHERE ed.type = ? AND src.fq_name = ?`
	return s.queryElements(query, model.EdgeCalls, fqName)
}

// GetContained returns elements contained by the named parent element.
func (s *SQLiteStore) GetContained(name string) ([]model.Element, error) {
	query := `SELECT DISTINCT e.* FROM elements e
		JOIN edges ed ON ed.to_id = e.id
		JOIN elements parent ON ed.from_id = parent.id
		WHERE ed.type = 'CONTAINS' AND (parent.fq_name = ? OR parent.name LIKE '%' || ? || '%')`
	return s.queryElements(query, name, name)
}

// Search searches for elements by name or FQName substring.
func (s *SQLiteStore) Search(substring string) ([]model.Element, error) {
	query := `SELECT * FROM elements WHERE name LIKE '%' || ? || '%' OR fq_name LIKE '%' || ? || '%'`
	return s.queryElements(query, substring, substring)
}

// GetAllElements returns all elements in the database.
func (s *SQLiteStore) GetAllElements() ([]model.Element, error) {
	return s.queryElements(`SELECT * FROM elements`)
}

// DeleteEdgesByType removes all edges of a given type.
func (s *SQLiteStore) DeleteEdgesByType(edgeType string) error {
	_, err := s.db.Exec(`DELETE FROM edges WHERE type = ?`, edgeType)
	if err != nil {
		return fmt.Errorf("delete edges by type: %w", err)
	}
	return nil
}

// RunSQL executes a raw SQL query and returns rows as maps.
func (s *SQLiteStore) RunSQL(query string) ([]map[string]any, error) {
	rows, err := s.db.Query(query)
	if err != nil {
		return nil, fmt.Errorf("run sql: %w", err)
	}
	defer func() { _ = rows.Close() }()

	cols, err := rows.Columns()
	if err != nil {
		return nil, fmt.Errorf("get columns: %w", err)
	}

	var results []map[string]any
	for rows.Next() {
		vals := make([]any, len(cols))
		ptrs := make([]any, len(cols))
		for i := range vals {
			ptrs[i] = &vals[i]
		}
		if err := rows.Scan(ptrs...); err != nil {
			return nil, fmt.Errorf("scan row: %w", err)
		}
		row := make(map[string]any, len(cols))
		for i, col := range cols {
			row[col] = vals[i]
		}
		results = append(results, row)
	}
	return results, rows.Err()
}

// queryElements executes a query and scans results into Element slices.
func (s *SQLiteStore) queryElements(query string, args ...any) ([]model.Element, error) {
	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, fmt.Errorf("query elements: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var elements []model.Element
	for rows.Next() {
		var el model.Element
		err := rows.Scan(
			&el.ID, &el.Language, &el.Kind, &el.Name, &el.FQName, &el.Path,
			&el.StartLine, &el.EndLine, &el.LOC, &el.Signature, &el.Visibility, &el.Docstring, &el.Body,
		)
		if err != nil {
			return nil, fmt.Errorf("scan element: %w", err)
		}
		elements = append(elements, el)
	}
	return elements, rows.Err()
}

// scanElement scans a single database row into an Element.
func scanElement(row *sql.Row) (model.Element, error) {
	var el model.Element
	err := row.Scan(
		&el.ID, &el.Language, &el.Kind, &el.Name, &el.FQName, &el.Path,
		&el.StartLine, &el.EndLine, &el.LOC, &el.Signature, &el.Visibility, &el.Docstring, &el.Body,
	)
	return el, err
}
