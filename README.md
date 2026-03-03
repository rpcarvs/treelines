# Treelines

Code intelligence CLI powered by Tree-sitter. Parses codebases, extracts structural
elements (functions, methods, classes, structs, interfaces, traits, enums, impl blocks, modules), maps their relationships,
and stores everything in a local SQLite graph for fast querying.

Supports **Go**, **Python**, and **Rust**.

## Quick Start

```bash
go install .
lines init
lines index
```

This creates a `.treelines/` directory with a `codestore.db` SQLite database.
Add `.treelines/` to your `.gitignore`. If `.gitignore` does not exist yet, create it first.

## Commands

### Setup

| Command | Description |
|---------|-------------|
| `lines init` | Create `.treelines/` directory and database schema |
| `lines index` | Full index of the codebase |
| `lines update` | Incremental re-index of files changed since last indexed commit |
| `lines serve` | Watch for file changes, re-index automatically, and refresh cross-file CALLS/IMPORTS/EXPORTS edges |

### Querying Elements

| Command | Description |
|---------|-------------|
| `lines element <name>` | Look up an element by FQName, exact name, or substring |
| `lines search <substring>` | Search symbols by name or FQName substring |
| `lines list <name>` | List elements contained by a named element (package, struct, etc) |
| `lines stats` | Show element and edge counts |
| `lines exports [module]` | Authoritative Python `__all__` export surface query |

### Querying Relationships

| Command | Description |
|---------|-------------|
| `lines uses <fq_name>` | Who calls this function? |
| `lines callees <fq_name>` | What does this function call? |

### Advanced

| Command | Description |
|---------|-------------|
| `lines query <sql>` | Run raw SQL against the database |
| `lines query --file <path>` | Read SQL from a file |
| `lines query --file -` | Read SQL from stdin |

Guidance:
- Use `search` for symbol lookup.
- Use `exports` for Python package surface (`__all__`).

## Global Flags

| Flag | Description |
|------|-------------|
| `--no-body` | Strip function/method bodies from element detail output |
| `--json` | Output as JSON instead of compact text |
| `--verbose` | Show detailed progress during indexing |
| `--quiet` | Suppress non-essential output |
| `--db <path>` | Override database path |

## Output Format

Default output is compact text optimized for token efficiency.

Lists show a header row followed by one line per element:
```
KIND       FQNAME                                   PATH                           VIS      LOC
function   cmd.runIndex                             cmd/index.go:26     private  87 loc
```

Single element detail shows structured metadata and body:
```
go function cmd.runIndex (private)
  cmd/index.go:26-112 (87 loc)
  func runIndex(cmd *cobra.Command, args []string) error {
  # description from docstring

func runIndex(cmd *cobra.Command, args []string) error {
    ...
}
```

Use `--no-body` to hide the source body. Use `--json` when you need machine-parseable output.

## Element Kinds

Values for `--kind` filters and the `kind` column in SQL:

`function`, `method`, `class`, `struct`, `interface`, `trait`, `enum`, `impl`, `module`

## Fully Qualified Names

Elements are identified by fully qualified names (FQName). The format depends on
the language:

- **Go:** `package.Function`, `package.Type.Method` (e.g., `graph.SQLiteStore.Open`)
- **Python:** `module.Class.method` with dot-separated paths (e.g., `scanner.Scanner.scan`)
- **Rust:** `crate::module::Type::method` with `::` separators

Use `lines search` to discover FQNames when you don't know the exact format.

## Examples

```bash
# Find a function by name
lines element "SQLiteStore.Open"

# List all public functions in a package
lines list graph --public --kind function

# What does runIndex call?
lines callees "cmd.runIndex"

# Who calls NewScanner?
lines uses "scanner.NewScanner"

# Search for anything matching "Path", only functions
lines search "Path" --kind function

# Get element metadata without the full body
lines element "graph.SQLiteStore" --no-body

# List Python modules with static __all__ exports
lines exports

# Show exported symbols for a module and include __all__ location
lines exports "__init__" --source
```

## Raw SQL Queries

The `query` command provides direct SQL access for analysis that the structured
commands don't cover. The database has two tables:

### elements

| Column | Type | Description |
|--------|------|-------------|
| id | TEXT | Deterministic hash ID |
| language | TEXT | `go`, `python`, or `rust` |
| kind | TEXT | Element kind (see list above) |
| name | TEXT | Short name |
| fq_name | TEXT | Fully qualified name |
| path | TEXT | Relative file path |
| start_line | INTEGER | First line |
| end_line | INTEGER | Last line |
| loc | INTEGER | Lines of code |
| signature | TEXT | First line of declaration |
| visibility | TEXT | `public` or `private` |
| docstring | TEXT | Extracted documentation |
| body | TEXT | Full source text |

### edges

| Column | Type | Description |
|--------|------|-------------|
| from_id | TEXT | Source element ID |
| to_id | TEXT | Target element ID |
| type | TEXT | Edge type |

Edge types: `CALLS`, `IMPORTS`, `EXPORTS`, `CONTAINS`, `DEFINED_IN`, `IMPLEMENTS`, `EXTENDS`

### SQL Examples

Shell quoting can make single quotes difficult. Use `--file -` to pipe SQL via stdin:

```bash
# Find the 10 largest functions
echo "SELECT fq_name, loc FROM elements WHERE kind = 'function' ORDER BY loc DESC LIMIT 10" \
  | lines query --file -

# Count elements per file
echo "SELECT path, COUNT(*) as cnt FROM elements GROUP BY path ORDER BY cnt DESC" \
  | lines query --file -

# Find functions that call more than 5 other functions
echo "SELECT e.fq_name, COUNT(*) as call_count FROM elements e JOIN edges ed ON ed.from_id = e.id WHERE ed.type = 'CALLS' GROUP BY e.id HAVING call_count > 5 ORDER BY call_count DESC" \
  | lines query --file -
```

## How It Works

1. **Scanning** - Walks the project tree, respecting `.gitignore`
2. **Parsing** - Tree-sitter parses each file into a syntax tree
3. **Extraction** - Language-specific extractors pull elements and intra-file edges
4. **Cross-reference** - A second pass resolves cross-file edges (`index`, `update`, and `serve`):
   `CALLS` for Go/Python/Rust, `IMPORTS` for internal Python imports, and
   `EXPORTS` for static Python `__all__` assignments
5. **Storage** - Everything goes into SQLite with indexes on name, FQName, and path
