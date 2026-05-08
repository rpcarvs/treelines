# Treelines

Treelines is a local code-intelligence CLI for Go, Python, and Rust.
It parses source files with Tree-sitter, stores symbols and relationships in SQLite, and provides compact deterministic queries for agents and humans.

## Table of Contents

- [Why Treelines](#why-treelines)
- [Quick Start](#quick-start)
- [Agent Workflow](#agent-workflow)
- [Install Agent Integration](#install-agent-integration)
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

Install with Homebrew:

```bash
brew tap rpcarvs/treelines
brew install treelines
treelines --version
```

Current packaged targets:
- macOS arm64
- Linux amd64

Homebrew formulas are rendered from native cgo release builds for those targets.

Install with Go:

```bash
go install github.com/rpcarvs/treelines@latest
treelines --version
```

If `treelines` is not found, make sure your Go bin directory is in `PATH`:

```bash
export PATH="$PATH:$(go env GOPATH)/bin"
```

For local development from this repository:

```bash
go install .
treelines --version
```

Initialize and index a Git repository:

```bash
cd /path/to/git/repo

treelines init
treelines index
treelines onboard
treelines overview
treelines stats
```

What this does:
- Resolves the Git repository root, even when run from a subdirectory
- Creates `<git-root>/.treelines/codestore.db`
- Initializes schema and indexes
- Builds a full code snapshot

Notes:
- Treelines requires a Git repository for project-scoped commands
- `treelines init` is idempotent and does not wipe indexed data
- `treelines index` performs full snapshot replacement (removed code is removed from DB)
- `treelines -v` and `treelines --version` report Go module version metadata for tagged module installs
- Local source builds may report `unknown (built from source)`
- `treelines init` adds `.treelines/` to `.gitignore` when `.gitignore` exists

## Agent Workflow

Recommended deterministic workflow when agents do not auto-commit:

1. `treelines init`
2. `treelines index` before coding starts
3. Use `treelines onboard` or `treelines recap` when the workflow is not fresh in context
4. Start with `treelines overview` for a compact first-pass map
5. Use targeted `treelines` commands for exploration and narrowing scope
6. Run `treelines index` again when you need a fresh post-edit snapshot

For git commit-based workflows, `treelines update` can be used in step 4 instead. It uses the last indexed git commit to update the database for only modified files instead of full index.

Alternatively, `treelines serve` starts a filesystem watcher that keeps the database updated from file-change events. It is usually only needed for large codebases or long-running manual sessions.

## Install Agent Integration

Treelines can install its agent integration for Codex or Claude:

```bash
treelines install codex
treelines install claude
```

Each provider install:
- Installs the shared `treelines-codebase-exploration` skill.
- Adds or updates the managed Treelines context block.
- Installs a `SessionStart` hook that runs `treelines init && treelines onboard` inside Git repositories.
- Prints all installed or updated paths.

Use `--local` to install into the current Git repository instead of the global agent config:

```bash
treelines install codex --local
treelines install claude --local
```

Use `--force` when you want to clear the existing installed skill directory before reinstalling:

```bash
treelines install codex --force
treelines install claude --force
```

Global targets:
- Codex skill: `$CODEX_HOME/skills/treelines-codebase-exploration` or `~/.codex/skills/treelines-codebase-exploration`
- Codex context: `$CODEX_HOME/AGENTS.md` or `~/.codex/AGENTS.md`
- Codex hooks: `$CODEX_HOME/hooks.json` or `~/.codex/hooks.json`
- Claude skill: `~/.claude/skills/treelines-codebase-exploration`
- Claude context: `~/.claude/CLAUDE.md`
- Claude hooks: `~/.claude/settings.json`

Local targets:
- Context block: repo-root `AGENTS.md`
- Claude pointer: repo-root `CLAUDE.md` containing `See [AGENTS.md](./AGENTS.md)`
- Skills and hooks: repo-root `.codex/` or `.claude/`

Context blocks are managed with internal markers and replaced on re-run.

## Command Reference

### Setup and Lifecycle

| Command | Purpose |
|---|---|
| `treelines init` | Create root `.treelines/` and initialize schema |
| `treelines index` | Full re-index snapshot |
| `treelines update` | Incremental re-index from `.treelines/last_commit` to git `HEAD` |
| `treelines serve` | Watch file changes and incrementally re-index (filesystem-event based) |
| `treelines stats` | Counts by kind, language, and edge type |
| `treelines overview [--depth 1\|2\|3]` | Compact first-pass codebase map optimized for agents |
| `treelines onboard` | Very short agent workflow reminder |
| `treelines recap` | Longer command and workflow recap |

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
| `treelines install codex [--local] [--force]` | Install Codex skill, context, and SessionStart hook |
| `treelines install claude [--local] [--force]` | Install Claude skill, context, and SessionStart hook |

Project-scoped commands resolve the current Git repository root and can be run from any subdirectory.

Global flags:
`--json`, `--no-body`, `--verbose`, `--quiet`, `--db <path>`, `-v`, `--version`

Use `treelines --help` and `treelines <command> --help` for command details.

## Examples

```bash
# Discovery
treelines stats
treelines onboard
treelines overview
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
