# Contributing

## Prerequisites

- Go 1.23+
- [Ginkgo](https://onsi.github.io/ginkgo/) test framework

## Development

Build the CLI:

```bash
go build -o gitlab-ci-helper ./cmd/gitlab-ci-helper
```

Run tests:

```bash
go install github.com/onsi/ginkgo/v2/ginkgo@latest
ginkgo -r --randomize-all --randomize-suites --race ./...
```

Lint:

```bash
go vet ./...
gofmt -l .
```

## CI/CD

- `.github/workflows/ci.yml` — runs lint, tests (ginkgo), and build on push to `master` and PRs.
- `.github/workflows/release.yml` — on `v*` tags: runs tests, builds the CLI binary, publishes GitHub release assets, and pushes the runner image to GHCR.

## Releasing

1. Update `docker/codex-base/CODEX_VERSION` if you want to pin the runner image to a specific `@openai/codex` npm version (defaults to `latest`).
2. Tag and push:

```bash
git tag v0.1.0
git push origin v0.1.0
```

The release workflow will automatically:
- Build the `gitlab-ci-helper` binary and attach it to the GitHub Release.
- Build and push `ghcr.io/miare-ir/gitlab-ci-helper-runner` with semver tags.

## Project layout

```
cmd/gitlab-ci-helper/    CLI entry point
internal/setup/          Setup wizard logic
templates/               GitLab CI templates and scripts
docker/codex-base/       Runner image Dockerfile
```
