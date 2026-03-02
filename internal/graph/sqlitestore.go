package graph

import (
	"database/sql"
	"fmt"

	"lines/internal/model"

	_ "modernc.org/sqlite"
)

// SQLiteStore implements GraphStore using SQLite via modernc.org/sqlite.
type SQLiteStore struct {
	db *sql.DB
}

func NewSQLiteStore() *SQLiteStore {
	return &SQLiteStore{}
}

func (s *SQLiteStore) Open(path string) error {
	db, err := sql.Open("sqlite", path+"?_busy_timeout=5000")
	if err != nil {
		return fmt.Errorf("open database: %w", err)
	}
	s.db = db
	return nil
}

func (s *SQLiteStore) Close() error {
	if s.db != nil {
		return s.db.Close()
	}
	return nil
}

func (s *SQLiteStore) CreateSchema() error {
	for _, stmt := range SchemaStatements() {
		if _, err := s.db.Exec(stmt); err != nil {
			return fmt.Errorf("create schema: %w", err)
		}
	}
	return nil
}

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

func (s *SQLiteStore) UpsertEdge(e model.Edge) error {
	query := `INSERT INTO edges (from_id, to_id, type) VALUES (?, ?, ?)
		ON CONFLICT(from_id, to_id, type) DO NOTHING`
	_, err := s.db.Exec(query, e.From, e.To, e.Type)
	if err != nil {
		return fmt.Errorf("upsert edge: %w", err)
	}
	return nil
}

func (s *SQLiteStore) DeleteElement(id string) error {
	_, err := s.db.Exec(`DELETE FROM elements WHERE id = ?`, id)
	if err != nil {
		return fmt.Errorf("delete element: %w", err)
	}
	return nil
}

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

func (s *SQLiteStore) GetElementByExactName(name string) ([]model.Element, error) {
	query := `SELECT * FROM elements WHERE name = ?`
	return s.queryElements(query, name)
}

func (s *SQLiteStore) GetElementsByName(name string) ([]model.Element, error) {
	query := `SELECT * FROM elements WHERE name LIKE '%' || ? || '%'`
	return s.queryElements(query, name)
}

func (s *SQLiteStore) GetCallers(fqName string) ([]model.Element, error) {
	query := `SELECT DISTINCT e.* FROM elements e
		JOIN edges ed ON ed.from_id = e.id
		JOIN elements t ON ed.to_id = t.id
		WHERE ed.type = ? AND t.fq_name = ?`
	return s.queryElements(query, model.EdgeCalls, fqName)
}

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

func (s *SQLiteStore) Search(substring string) ([]model.Element, error) {
	query := `SELECT * FROM elements WHERE name LIKE '%' || ? || '%' OR fq_name LIKE '%' || ? || '%'`
	return s.queryElements(query, substring, substring)
}

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

func scanElement(row *sql.Row) (model.Element, error) {
	var el model.Element
	err := row.Scan(
		&el.ID, &el.Language, &el.Kind, &el.Name, &el.FQName, &el.Path,
		&el.StartLine, &el.EndLine, &el.LOC, &el.Signature, &el.Visibility, &el.Docstring, &el.Body,
	)
	return el, err
}
