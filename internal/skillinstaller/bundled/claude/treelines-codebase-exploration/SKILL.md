---
name: treelines-codebase-exploration
description: Use treelines BEFORE any Python, Rust, or Go source code exploration -- for understanding structure, discovering elements, tracing dependencies, or answering questions about the codebase. Saves tokens and gives deterministic results.
---

# Treelines Codebase Exploration

## Core Rule

Use `treelines` first for any Python, Rust, or Go source code exploration before reading files directly. `treelines` indexes source code into a SQLite graph DB using tree-sitter, giving deterministic structural results with significant token savings.

- Start with `treelines --help` when command usage is uncertain.
- Query structure first, then read only files that are still needed.
- If findings are insufficient, do targeted file reads and return to `treelines` queries.

## Required Workflow (No serve)

Run this sequence at the start of any session involving Python, Rust, or Go codebases.

1. `treelines index` before work starts
2. Use `treelines` commands for exploration and dependency mapping
  - treelines stats
  - treelines list . --kind module
  - treelines search <entry symbol>
  - treelines callees <entry symbol> or treelines uses <symbol>
3. Run `treelines index` again when a fresh post-edit snapshot is needed

Do not use `update` or `serve` in this workflow.

## Command Reference

When in doubt, run `treelines --help` and `treelines <command> --help`.

- Setup:
  - `treelines init`
  - `treelines index`
  - `treelines install codex-skill`
  - `treelines install claude-skill`
- Discovery:
  - `treelines stats`
  - `treelines list . --kind module`
  - `treelines search Scanner`
  - `treelines element graph.SQLiteStore.Open`
- Relationships:
  - `treelines callees cmd.runIndex`
  - `treelines uses graph.SQLiteStore.Open`
  - `treelines module-graph` (repo overview)
  - `treelines module-graph cmd` (single module)
- Dependency surface:
  - `treelines imports`
  - `treelines imports cmd`
  - `treelines exports`
  - `treelines exports __init__ --source`
- Advanced SQL:
  - `treelines query --schema`
  - `treelines query "SELECT kind, COUNT(*) FROM elements GROUP BY kind"`

## Practical Guidance

- Prefer `--json` only for machine parsing and deterministic downstream processing.
- Use `--no-body` when body text is not required.
- Keep queries compact to reduce token usage.
- Treat `treelines` as structural ground truth, then validate behavior with selective file reads when needed.

## Integration with Other Skills

`treelines` is not an alternative to other skills.

**The failure mode:** Another skill's workflow feels complete on its own, so you skip treelines. This is wrong.

## Subagent Enforcement

When spawning subagents via the Agent tool, always include the full content of this skill in the subagent's prompt. Subagents do not inherit skills automatically. If a subagent will read, understand, write or modify code, it may need `treelines`.
