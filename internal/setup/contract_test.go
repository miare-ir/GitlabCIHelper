package setup

import (
	"os"
	"path/filepath"
	"runtime"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Template and script contracts", func() {
	It("template uses known env contract", func() {
		templateBody := mustReadRepoFile("templates/gitlab-ci-helper.yml")
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
			Expect(templateBody).To(ContainSubstring(key))
		}
	})

	It("template compile-time fields avoid shell-style fallback expansion", func() {
		templateBody := mustReadRepoFile("templates/gitlab-ci-helper.yml")
		lines := strings.Split(templateBody, "\n")
		for _, line := range lines {
			trimmed := strings.TrimSpace(line)
			switch {
			case strings.HasPrefix(trimmed, "stage:"):
				Expect(trimmed).NotTo(ContainSubstring("$"))
			case strings.HasPrefix(trimmed, "image:"):
				Expect(trimmed).NotTo(ContainSubstring("${"))
			}
		}
	})

	It("scripts use known env contract", func() {
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
			body := mustReadRepoFile(relPath)
			for _, key := range keys {
				Expect(body).To(ContainSubstring(key))
			}
		}
	})
})

func mustReadRepoFile(relativePath string) string {
	_, currentFile, _, ok := runtime.Caller(0)
	Expect(ok).To(BeTrue())

	repoRoot := filepath.Clean(filepath.Join(filepath.Dir(currentFile), "..", ".."))
	body, err := os.ReadFile(filepath.Join(repoRoot, relativePath))
	Expect(err).NotTo(HaveOccurred())

	return string(body)
}
