# gitlab-ci-helper

Reusable GitLab CI package for cross-project automation.

## Included jobs

- `gitlab_ci_helper_auto_open_mr` — automatically opens merge requests.
- `gitlab_ci_helper_codex_review` — AI-powered MR review integrated with GitLab Discussions API.

## Installation

Download the latest binary from [Releases](https://github.com/miare-ir/GitlabCIHelper/releases):

```bash
curl -fsSL https://github.com/miare-ir/GitlabCIHelper/releases/latest/download/gitlab-ci-helper -o gitlab-ci-helper
chmod +x gitlab-ci-helper
```

## Setup

Run the interactive wizard from the root of your GitLab project (where `.gitlab-ci.yml` exists):

```bash
./gitlab-ci-helper setup
```

The wizard will:

- inspect `.gitlab-ci.yml` and local include chains,
- prompt for per-job trigger/stage and optional override paths,
- preview diffs before writing,
- update `.gitlab-ci.yml` and `.gitlab-ci-helper/config.yml`,
- sync helper assets into `.gitlab-ci-helper/`.

Commit the `.gitlab-ci-helper/` directory to your repository so the included jobs and scripts are available in CI.

## Required GitLab CI variables

Set these in the target project's **Settings > CI/CD > Variables**:

| Variable | Description |
|---|---|
| `GITLAB_CI_HELPER_TOKEN` | GitLab API token with access to the project |
| `GITLAB_CI_HELPER_CODEX_AUTH` | File variable; required when `codex_review` is enabled |
| `GITLAB_CI_HELPER_CODEX_IMAGE` | Optional override for the runner image (default: `ghcr.io/miare-ir/gitlab-ci-helper-runner:v0`) |

Pin `GITLAB_CI_HELPER_CODEX_IMAGE` to a concrete release tag (e.g. `ghcr.io/miare-ir/gitlab-ci-helper-runner:v0.1.0`) for reproducible pipelines.

## Runner image

The `codex_review` job runs inside a container image published to GitHub Container Registry:

```
docker pull ghcr.io/miare-ir/gitlab-ci-helper-runner:v0
```

Each release publishes tags `vX.Y.Z`, `vX.Y`, `vX`, and `latest`.
