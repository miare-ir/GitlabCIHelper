# gitlab_ci_helper - Agent Instructions

This repository ships a reusable GitLab CI helper package and an interactive Go setup wizard.

## Project overview

- Primary goal: configure helper CI jobs in target repositories through an interactive `setup` flow.
- CLI entrypoint: `cmd/gitlab-ci-helper/`.
- Core setup logic: `internal/setup/`.
- Shipped assets/templates: `templates/` embedded via `template_assets.go`.

## High-priority constraints

- Keep the wizard interactive in v1. Do not convert `setup` into a non-interactive-only flow.
- Never store secret values in repository-tracked config. Store only variable names, keys, or file variable paths.
- Preserve idempotency when modifying `.gitlab-ci.yml` (no duplicate include blocks, stable repeated runs).
- Keep `templates/codex/review_prompt.md` and `templates/codex/review_output_schema.json` compatible.
- Shell scripts must use `set -euo pipefail`.

## Repository map

- `cmd/gitlab-ci-helper/main.go`: CLI command wiring.
- `internal/setup/`: wizard UX, discovery, planning, YAML patching, diff preview, apply/write flow.
- `templates/gitlab-ci-helper.yml`: includeable CI job template.
- `templates/scripts/*.sh`: helper shell scripts used by CI jobs.
- `templates/codex/*`: codex review prompt and schema defaults.
- `template_assets.go`: embed patterns for all shipped assets.

## Setup and validation commands

Run from repository root:

- Build CLI: `go build -o gitlab-ci-helper ./cmd/gitlab-ci-helper`
- Run wizard: `./gitlab-ci-helper setup`
- Run tests (required after changes): `go test ./...`

## Coding and change conventions

- Keep edits focused and minimal; avoid broad refactors unless required.
- Prefer deterministic output for generated/updated YAML and config content.
- Maintain backward compatibility of config shape unless explicitly changing versioned behavior.
- When changing files under `templates/`, verify embedded paths in `template_assets.go` still cover all required files.
- When changing setup flow behavior, update tests in `internal/setup/` or add new coverage.

## Testing expectations

- Always run `go test ./...` before concluding.
- Add or adjust tests when behavior changes (especially discovery, YAML patching, and idempotent updates).
- If test fixtures include YAML/config content, keep them representative of real GitLab CI include chains.

## Security and secrets

- Do not hardcode tokens, auth payloads, or example secret values in tracked files.
- Preserve file-variable semantics for credentials (for example, Codex auth file variable patterns).
- Treat `.gitlab-ci-helper/config.yml` as non-secret metadata only.

## PR/change checklist for agents

- Scope is limited to requested behavior.
- Interactive `setup` flow still works.
- `.gitlab-ci.yml` updates remain idempotent.
- Prompt/schema compatibility in `templates/codex/` preserved.
- `go test ./...` passes.

## Nested instructions

Additional `AGENTS.md` files exist under `cmd/gitlab-ci-helper/`, `internal/setup/`, and `templates/`. When editing in those directories, follow the nearest file first.
