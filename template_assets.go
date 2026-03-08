package gitlab_ci_helper

import "embed"

// EmbeddedTemplates contains the helper templates shipped with the binary.
//
//go:embed templates/gitlab-ci-helper.yml templates/mr_description.md templates/scripts/*.sh templates/codex/*
var EmbeddedTemplates embed.FS
