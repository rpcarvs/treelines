package graph

// SchemaStatements returns SQL DDL statements for the elements and edges tables.
func SchemaStatements() []string {
	return []string{
		`PRAGMA journal_mode=WAL`,
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
		`CREATE TABLE IF NOT EXISTS edges (
			from_id TEXT NOT NULL,
			to_id TEXT NOT NULL,
			type TEXT NOT NULL,
			PRIMARY KEY (from_id, to_id, type)
		)`,
		`CREATE INDEX IF NOT EXISTS idx_elements_fq_name ON elements(fq_name)`,
		`CREATE INDEX IF NOT EXISTS idx_elements_name ON elements(name)`,
		`CREATE INDEX IF NOT EXISTS idx_elements_path ON elements(path)`,
		`CREATE INDEX IF NOT EXISTS idx_edges_from ON edges(from_id)`,
		`CREATE INDEX IF NOT EXISTS idx_edges_to ON edges(to_id)`,
		`CREATE INDEX IF NOT EXISTS idx_edges_type ON edges(type)`,
	}
}
