# Treelines

Treelines is a local code-intelligence CLI for Go, Python, and Rust.
It parses source files with Tree-sitter, stores symbols and relationships in SQLite, and provides compact deterministic queries for agents and humans.

## Table of Contents

- [Why Treelines](#why-treelines)
- [Quick Start](#quick-start)
- [Agent Workflow](#agent-workflow)
- [Install Skill and Context](#install-skill-and-context)
- [Command Reference](#command-reference)
- [Examples](#examples)
- [Data Model](#data-model)
- [How It Works](#how-it-works)
- [Notes and Limits](#notes-and-limits)

## Why Treelines

- Fast structural discovery before expensive file reads
- Can provide massive token savings for agents
- Deterministic local graph database (no remote service)
- Compact CLI output designed for token-efficient workflows
- Works across Go, Python, and Rust with a common query surface

## Quick Start

```bash
# Install directly from GitHub:
go install github.com/rpcarvs/treelines@latest

# Or clone the repo and install locally:
go install .
# If `treelines` is not found, add GOPATH/bin to PATH (Bash example):
grep -q '$(go env GOPATH)/bin' ~/.bashrc || echo 'export PATH="$PATH:$(go env GOPATH)/bin"' >> ~/.bashrc
source ~/.bashrc
treelines init
treelines index
treelines stats
```

```bash
go install .
#
treelines init
treelines index
treelines stats
```

What this does:
- Creates `.treelines/codestore.db`
- Initializes schema and indexes
- Builds a full code snapshot

Notes:
- `treelines init` is idempotent and does not wipe indexed data
- `treelines index` performs full snapshot replacement (removed code is removed from DB)
- Add `.treelines/` to `.gitignore`

## Agent Workflow

Recommended deterministic workflow when agents do not auto-commit:

1. `treelines init`
2. `treelines index` before coding starts
3. Use `treelines` commands first for exploration and narrowing scope
4. Run `treelines index` again when you need a fresh post-edit snapshot

For git commit-based workflows, `treelines update` can be used in step 4 instead. It uses the last indexed git commit to update the database for only modified files instead of full index.

Alternatively, a `treelines serve` creates a "daemon" that constantly update modified files. Probably only interesting for large codebases.

## Install Skill and Context

Install bundled skill:

```bash
treelines install codex-skill
treelines install claude-skill
```

Install or refresh managed context policy block:

```bash
# global
treelines install codex-context
treelines install claude-context

# project-local
treelines install codex-context --local
treelines install claude-context --local
```

Context targets:
- global: `~/.codex/AGENTS.md`, `~/.claude/CLAUDE.md`
- local: `./AGENTS.md`, `./CLAUDE.md`

Context blocks are managed and replaced by internal markers on re-run.

## Command Reference

### Setup and Lifecycle

| Command | Purpose |
|---|---|
| `treelines init` | Create `.treelines/` and initialize schema |
| `treelines index` | Full re-index snapshot |
| `treelines update` | Incremental re-index from `.treelines/last_commit` to git `HEAD` |
| `treelines serve` | Watch file changes and incrementally re-index (filesystem-event based) |
| `treelines stats` | Counts by kind, language, and edge type |

### Discovery

| Command | Purpose |
|---|---|
| `treelines search <substring>` | Symbol-oriented name/FQName search |
| `treelines element <name>` | FQName > exact short name > substring lookup |
| `treelines list <name\|.\|*>` | Contained elements; `.` or `*` means repo-wide scope |

### Relationships

| Command | Purpose |
|---|---|
| `treelines callees <fq_name>` | Outgoing calls from an element |
| `treelines uses <fq_name>` | Incoming callers of an element |
| `treelines imports [module]` | Internal import dependencies |
| `treelines exports [module]` | Export surface (Python `__all__`, Go/Rust public symbols) |
| `treelines module-graph [module]` | Module summary, or repo overview without args |

### Advanced

| Command | Purpose |
|---|---|
| `treelines query <sql>` | Execute raw SQL |
| `treelines query --file <path>` | Read SQL from file |
| `treelines query --file -` | Read SQL from stdin |
| `treelines query --schema` | Print schema and sample queries |

### Installers

| Command | Purpose |
|---|---|
| `treelines install codex-skill` | Install bundled Codex skill |
| `treelines install claude-skill` | Install bundled Claude skill |
| `treelines install codex-context [--local]` | Install/update Codex context policy block |
| `treelines install claude-context [--local]` | Install/update Claude context policy block |

Global flags:
`--json`, `--no-body`, `--verbose`, `--quiet`, `--db <path>`

Use `treelines --help` and `treelines <command> --help` for command details.

## Examples

```bash
# Discovery
treelines stats
treelines list . --kind module
treelines search "Scanner"
treelines element "graph.SQLiteStore.Open"

# Relationships
treelines callees "cmd.runIndex"
treelines uses "graph.SQLiteStore.Open"
treelines imports "cmd"
treelines module-graph
treelines module-graph "cmd"

# Export surface
treelines exports
treelines exports "crate::ml"
treelines exports "__init__" --source

# SQL
treelines query --schema
echo "SELECT kind, COUNT(*) AS c FROM elements GROUP BY kind ORDER BY c DESC" | treelines query --file -
```

## Data Model

Treelines stores data in two SQLite tables.

### elements

| Column |
|---|
| `id` |
| `language` |
| `kind` |
| `name` |
| `fq_name` |
| `path` |
| `start_line` |
| `end_line` |
| `loc` |
| `signature` |
| `visibility` |
| `docstring` |
| `body` |

### edges

| Column |
|---|
| `from_id` |
| `to_id` |
| `type` |

Edge types:
`CALLS`, `IMPORTS`, `EXPORTS`, `CONTAINS`, `DEFINED_IN`, `IMPLEMENTS`, `EXTENDS`

Element kinds:
`function`, `method`, `class`, `struct`, `interface`, `trait`, `enum`, `impl`, `module`

FQName formats:
- Go: `pkg.Func`, `pkg.Type.Method`
- Python: `module.Class.method`
- Rust: `crate::module::Type::method`

## How It Works

1. Scan files while honoring `.gitignore`
2. Parse syntax trees with Tree-sitter
3. Extract elements and intra-file edges per language
4. Resolve cross-file edges (`CALLS`, internal `IMPORTS`, Python static `EXPORTS`)
5. Persist to SQLite with indexed lookups

## Notes and Limits

- `search` is symbol-oriented, not generic text grep
- `exports` is language-aware; Go/Rust exports are module-local, non-recursive
- `update` depends on git commit markers and does not include unstaged or uncommitted changes
- `serve` is not git-dependent
