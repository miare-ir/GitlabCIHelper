package setup

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestTemplateUsesKnownEnvContract(t *testing.T) {
	t.Parallel()

	templateBody := mustReadRepoFile(t, "templates/gitlab-ci-helper.yml")
	required := []string{
		EnvAutoOpenMREnabled,
		EnvAutoOpenMRTriggerMode,
		EnvCodexReviewEnabled,
		EnvCodexReviewTriggerMode,
		EnvCodexReviewAllowFailure,
		EnvCodexImage,
		autoOpenMRStagePlaceholder,
		codexReviewStagePlaceholder,
		TriggerManualAnyBranch,
		TriggerManualNonDefault,
		TriggerAlwaysNonDefault,
		TriggerManualMREvent,
		TriggerAlwaysMREvent,
	}
	for _, key := range required {
		if !strings.Contains(templateBody, key) {
			t.Fatalf("expected templates/gitlab-ci-helper.yml to reference %q", key)
		}
	}
}

func TestTemplateCompileTimeFieldsAvoidShellStyleFallbackExpansion(t *testing.T) {
	t.Parallel()

	templateBody := mustReadRepoFile(t, "templates/gitlab-ci-helper.yml")
	lines := strings.Split(templateBody, "\n")
	for _, line := range lines {
		trimmed := strings.TrimSpace(line)
		switch {
		case strings.HasPrefix(trimmed, "stage:"):
			if strings.Contains(trimmed, "$") {
				t.Fatalf("stage must be concrete in template, got line: %q", trimmed)
			}
		case strings.HasPrefix(trimmed, "image:"):
			if strings.Contains(trimmed, "${") {
				t.Fatalf("image must not use shell-style fallback expansion in template, got line: %q", trimmed)
			}
		}
	}
}

func TestScriptsUseKnownEnvContract(t *testing.T) {
	t.Parallel()

	tests := map[string][]string{
		"templates/scripts/auto_open_mr.sh": {
			EnvToken,
			EnvMRTemplatePath,
		},
		"templates/scripts/run_codex_review.sh": {
			EnvToken,
			EnvCodexAuth,
			EnvCodexReviewModel,
			EnvCodexPromptPath,
			EnvCodexSchemaPath,
		},
		"templates/scripts/codex_to_gitlab_discussions.sh": {
			EnvToken,
		},
	}

	for relPath, keys := range tests {
		body := mustReadRepoFile(t, relPath)
		for _, key := range keys {
			if !strings.Contains(body, key) {
				t.Fatalf("expected %s to reference %q", relPath, key)
			}
		}
	}
}

func mustReadRepoFile(t *testing.T, relativePath string) string {
	t.Helper()
	_, currentFile, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("failed to resolve test file path")
	}
	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	body, err := os.ReadFile(filepath.Join(repoRoot, relativePath))
	if err != nil {
		t.Fatalf("read %s: %v", relativePath, err)
	}
	return string(body)
}
