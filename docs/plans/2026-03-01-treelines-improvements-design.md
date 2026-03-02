# Treelines Improvements Design

## Goal

Make treelines useful for agents by providing enough detail to skip file reads, fixing structural accuracy, and enabling cross-package relationship discovery.

## 1. Store element source body

Add `body TEXT` column to `elements` table and `Body string` field to `model.Element`. Each extractor populates it with `node.Utf8Text(source)` during extraction. An agent querying an element gets the full source inline.

## 2. Fix FQName generation

- Go: use actual `package` name as prefix (e.g., `graph.SQLiteStore.Open` not `main.SQLiteStore.Open`)
- Python: derive from file path relative to root (e.g., `auth.validators.Validator`)
- Rust: keep current `crate::module::name` scheme

## 3. Smarter element lookup

Priority order in the `element` command: exact FQName match, then exact name match, then substring search.

## 4. Post-index cross-package call resolution

After initial indexing, run a second pass using the full element database as resolver. Re-parse files, walk call expressions, resolve against all known elements, insert CALLS edges with full cross-package visibility. Applies to both `lines index` and `lines update`.

## 5. Database filename

Default path changes from `.treelines/db` to `.treelines/codestore.db`.
