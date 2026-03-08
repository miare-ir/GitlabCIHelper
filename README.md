# gitlab-ci-helper

Reusable GitLab CI package for cross-project automation.

## Included jobs

- `gitlab_ci_helper_auto_open_mr`
- `gitlab_ci_helper_codex_review`

`re_open_release_mr` is intentionally out of v1 and reserved in config as a disabled placeholder.

## Wizard

Build and run:

```bash
go build -o gitlab-ci-helper ./cmd/gitlab-ci-helper
gitlab-ci-helper setup
```

The wizard:

- inspects `.gitlab-ci.yml` and local include chains,
- prompts for per-job trigger/stage and optional override paths,
- previews diffs before writing,
- updates `.gitlab-ci.yml` and `.gitlab-ci-helper/config.yml`,
- syncs standalone helper assets into `.gitlab-ci-helper/`,
- never stores secret values in repository files.

Commit `.gitlab-ci-helper/` to the target repository so included jobs/scripts are available in CI.

## Required GitLab CI variables

Set these in the target project:

- `GITLAB_CI_HELPER_TOKEN`
- `GITLAB_CI_HELPER_CODEX_AUTH` (file variable; only needed when `codex_review` is enabled)

## Template layout

- `templates/gitlab-ci-helper.yml`
- `templates/scripts/*.sh`
- `templates/codex/*`
- `templates/mr_description.md`
