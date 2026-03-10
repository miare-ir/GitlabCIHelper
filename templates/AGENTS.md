# AGENTS.md - templates

Scope: files in `templates/` and subdirectories.

## Purpose

- Contains shipped CI assets copied/embedded for downstream repositories.
- Includes CI YAML template, shell scripts, MR description, and Codex prompt/schema defaults.

## Critical invariants

- `templates/codex/review_prompt.md` and `templates/codex/review_output_schema.json` must remain compatible.
- Shell scripts must include `set -euo pipefail`.
- Template changes should remain safe for multi-project reuse.
- In CI YAML job compile-time fields (especially `stage`, `image`, `include`), avoid shell-style `${VAR:-default}` and unresolved dynamic values; keep values compile-safe (`stage` must be concrete by template render time).
- Never introduce real secrets or sensitive sample values.

## Embedding sync rule

- If files are added/renamed/removed in `templates/`, verify embed patterns in `template_assets.go` still include all required assets.

## Validation

- Run `ginkgo -r ./...` (or `go test ./...`) from repo root.
- If template behavior changed, sanity-check setup flow still syncs/copies expected files.

## Known pitfalls

- **Auto-generated headers belong directly in template source files**, not injected at runtime in Go code. Each template file should have the appropriate comment header (`#` for YAML/shell, `<!-- -->` for markdown). JSON files cannot have comments, so `review_output_schema.json` is excluded.
- **Shell script headers go after the shebang line**, not before it. The shebang (`#!/usr/bin/env bash`) must remain the first line for the script to be executable.
