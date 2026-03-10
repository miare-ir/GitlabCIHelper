# AGENTS.md - cmd/gitlab-ci-helper

Scope: files in `cmd/gitlab-ci-helper/`.

## Purpose

- Hosts the CLI entrypoint and command wiring.
- Must continue to expose the interactive `setup` command for v1 behavior.

## Rules for changes

- Keep CLI behavior simple and predictable.
- Do not move setup business logic into `cmd/`; keep orchestration in `internal/setup/`.
- Preserve existing command names/UX unless explicitly requested.
- Maintain clear error messages for failed setup execution.

## Validation

- Run `go test ./...` from repo root after changes.
- If command wiring changes, build with `go build -o gitlab-ci-helper ./cmd/gitlab-ci-helper`.
