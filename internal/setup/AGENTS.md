# AGENTS.md - internal/setup

Scope: files in `internal/setup/`.

## Purpose

- Implements the interactive setup wizard and write/apply workflow.
- Handles CI discovery, include/config planning, YAML patching, diff preview, and apply.

## Critical invariants

- Keep wizard interaction in v1 (prompt-driven flow remains first-class).
- `.gitlab-ci.yml` patching must be idempotent: repeated runs should not duplicate include blocks.
- Preserve deterministic patch output and stable ordering where possible.
- Never write secret values into repository config; keep only variable names/paths and non-secret metadata.

## Implementation guidance

- Keep discovery, planning, patching, and apply concerns separated.
- Prefer additive, low-risk changes in parser/patcher logic.
- When behavior changes, update tests near the affected component (`*_test.go`, contract tests, setup tests).
- Keep diff preview trustworthy: it should match what apply will write.

## Validation

- Run `go test ./...` from repo root.
- For patch logic changes, include/adjust tests that cover duplicate-prevention and repeated-run stability.
