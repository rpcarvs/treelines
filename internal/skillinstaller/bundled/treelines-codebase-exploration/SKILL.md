---
name: treelines-codebase-exploration
description: Use treelines BEFORE any Python, Rust, or Go source code exploration. Provides deterministic structure, symbols, imports, exports, and call relationships with compact output before selective file reads.
---

# Treelines Codebase Exploration

## Core Rule

Use `treelines` before exploring Python, Rust, or Go source code.

- Do not start codebase understanding with file reads, broad grep, glob scans, or directory walks.
- Use `treelines` as the first structural source, then read only the files that still matter.
- Direct file reads are allowed after `treelines` narrows the scope or when behavior cannot be represented structurally.
- If command usage is uncertain, use `treelines --help` or `treelines <command> --help`.
- Use `treelines recap` only when you need a fuller workflow or command reminder.

## Mandatory Lifecycle

Follow this sequence for any Python, Rust, or Go codebase exploration.

1. Orient:
- Run `treelines onboard` unless this workflow is already fresh in context.

2. Build a fresh snapshot:
- Run `treelines index`.
- Wait until indexing finishes before running any other `treelines` command.
- Never run `treelines index` in parallel with other `treelines` commands.

3. Get the broad map:
- Run `treelines overview`.
- Use `treelines overview --depth 3` when a deeper agent-facing summary is useful.

4. Narrow with targeted graph queries:
- Use `list`, `search`, `element`, `module-graph`, `imports`, `exports`, `callees`, and `uses` before reading files.
- Prefer compact, specific queries over repeated broad file reads.

5. Read files selectively:
- Read only the files or ranges needed for behavior, implementation details, edits, or validation.
- Return to `treelines` after edits or when relationship questions arise.

6. Refresh when needed:
- Run `treelines index` again after meaningful edits when you need a fresh structural snapshot.

Do not use `treelines update` or `treelines serve` in the default agent workflow.

## Command Decision Rules

- Need repository overview: `treelines overview`
- Need deeper overview: `treelines overview --depth 3`
- Need modules, classes, functions, structs, traits, enums: `treelines list . --kind <kind>`
- Need contents of a module or namespace: `treelines list <module>`
- Need to find a symbol: `treelines search <substring>`
- Need exact symbol details: `treelines element <fq_name>`
- Need module-level dependency summary: `treelines module-graph` or `treelines module-graph <module>`
- Need import dependencies: `treelines imports` or `treelines imports <module>`
- Need public/exported surface: `treelines exports` or `treelines exports <module>`
- Need what a function or method calls: `treelines callees <fq_name>`
- Need who calls a function or method: `treelines uses <fq_name>`
- Need custom compact output: `treelines query --schema`, then `treelines query "<sql>"`

## Efficient Exploration Pattern

Use this pattern to minimize tokens and avoid blind reads.

1. Start with `treelines overview`.
2. Use `treelines list . --kind module` to identify major areas.
3. Use `treelines module-graph <module>` before reading a module file.
4. Use `treelines search <symbol>` and `treelines element <fq_name>` before opening symbol definitions.
5. Use `treelines callees <fq_name>` and `treelines uses <fq_name>` before manually following call chains.
6. Read source files only after the graph identifies the relevant paths.

## Freshness Rules

- At session start, assume the database may be stale and run `treelines index`.
- After code edits, assume relationships may be stale until `treelines index` runs again.
- If results contradict visible source, rerun `treelines index` once before concluding the graph is wrong.
- If another process is indexing, wait and retry in 10 seconds.
- Do not run multiple indexing commands in parallel.

## File Read Boundaries

File reads are appropriate for:

- Behavior-critical implementation details.
- Runtime configuration, SQL, dataflow, macros, reflection, or dynamic behavior.
- Exact edits.
- Verifying a surprising or incomplete graph result.
- Reading non-Go, non-Python, or non-Rust assets that `treelines` does not index.

File reads are not appropriate as the first step for:

- Listing files and modules.
- Finding symbol names.
- Mapping imports or exports.
- Finding callers or callees.
- Building a first-pass architecture overview.

## Command Reference

Primary:
- `treelines onboard`
- `treelines index`
- `treelines overview`
- `treelines recap`

Discovery:
- `treelines list . --kind module`
- `treelines list <module>`
- `treelines search <substring>`
- `treelines element <fq_name>`

Relationships:
- `treelines module-graph`
- `treelines module-graph <module>`
- `treelines callees <fq_name>`
- `treelines uses <fq_name>`

Dependency surfaces:
- `treelines imports`
- `treelines imports <module>`
- `treelines exports`
- `treelines exports <module>`
- `treelines exports <module> --source`

Advanced:
- `treelines stats`
- `treelines query --schema`
- `treelines query "<sql>"`

Install:
- `treelines install codex-skill`
- `treelines install claude-skill`
- `treelines install codex-context`
- `treelines install claude-context`

## Output Rules

- Prefer normal text output for human-readable summaries.
- Use `--json` only when another tool or deterministic parser consumes the output.
- Use `--no-body` when bodies are not needed.
- Keep SQL queries small and explicit.
- Prefer graph commands over raw SQL unless the CLI lacks the exact view needed.

## Anti-Patterns

- Do not start by reading all files.
- Do not use broad grep as a substitute for `treelines search`.
- Do not use filesystem walks as a substitute for `treelines list . --kind module`.
- Do not rely on stale results after edits.
- Do not treat `treelines` as a full runtime or semantic engine.
- Do not ignore missing or surprising call edges. Verify with targeted file reads when necessary.

## Failure Protocol

- If `.treelines` or the database is missing, run `treelines init`, then `treelines index`.
- If a command fails because another writer is active, wait 10 seconds and retry.
- If command syntax is unclear, run `treelines <command> --help`.
- If command choice is unclear, run `treelines recap`.
- If the graph output is insufficient, read the smallest relevant source section and continue from there.
- If the graph appears stale, run `treelines index` before trusting the result.

## Subagent Enforcement

Subagents do not automatically inherit this skill.

When assigning subagents codebase work:

- Include this skill text or a strict summary in the subagent prompt.
- Require `treelines index` before exploration unless a fresh index is already guaranteed.
- Require `treelines overview` before file reads.
- Pass relevant `treelines` outputs when assigning a narrow module or symbol task.
- Tell subagents to use targeted file reads only after `treelines` narrows the scope.
