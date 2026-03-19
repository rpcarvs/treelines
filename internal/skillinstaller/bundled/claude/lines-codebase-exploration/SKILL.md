---
name: lines-codebase-exploration
description: Use lines BEFORE any Python, Rust, or Go source code exploration -- for understanding structure, discovering elements, tracing dependencies, or answering questions about the codebase. Saves tokens and gives deterministic results.
---

# Lines Codebase Exploration

## Core Rule

Use `lines` first for any Python, Rust, or Go source code exploration before reading files directly. `lines` indexes source code into a SQLite graph DB using tree-sitter, giving deterministic structural results with significant token savings.

- Start with `lines --help` when command usage is uncertain.
- Query structure first, then read only files that are still needed.
- If findings are insufficient, do targeted file reads and return to `lines` queries.

## Required Workflow (No serve)

Run this sequence at the start of any session involving Python, Rust, or Go codebases.

1. `lines init`
2. `lines index` before work starts
3. Use `lines` commands for exploration and dependency mapping
  - lines stats
  - lines list . --kind module
  - lines search <entry symbol>
  - lines callees <entry symbol> or lines uses <symbol>
4. Run `lines index` again when a fresh post-edit snapshot is needed

Do not use `update` or `serve` in this workflow.

## Command Reference

When in doubt, run `lines --help` and `lines <command> --help`.

- Setup:
  - `lines init`
  - `lines index`
  - `lines install codex-skill`
  - `lines install claude-skill`
- Discovery:
  - `lines stats`
  - `lines list . --kind module`
  - `lines search Scanner`
  - `lines element graph.SQLiteStore.Open`
- Relationships:
  - `lines callees cmd.runIndex`
  - `lines uses graph.SQLiteStore.Open`
  - `lines module-graph` (repo overview)
  - `lines module-graph cmd` (single module)
- Dependency surface:
  - `lines imports`
  - `lines imports cmd`
  - `lines exports`
  - `lines exports __init__ --source`
- Advanced SQL:
  - `lines query --schema`
  - `lines query "SELECT kind, COUNT(*) FROM elements GROUP BY kind"`

## Practical Guidance

- Prefer `--json` only for machine parsing and deterministic downstream processing.
- Use `--no-body` when body text is not required.
- Keep queries compact to reduce token usage.
- Treat `lines` as structural ground truth, then validate behavior with selective file reads when needed.

## Integration with Other Skills

`lines` is not an alternative to other skills.

**The failure mode:** Another skill's workflow feels complete on its own, so you skip lines. This is wrong.

## Subagent Enforcement

When spawning subagents via the Agent tool, always include the full content of this skill in the subagent's prompt. Subagents do not inherit skills automatically. If a subagent will read, understand, write or modify code, it may need `lines`.
