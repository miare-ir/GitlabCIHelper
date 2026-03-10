package setup

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"gopkg.in/yaml.v3"
)

var _ = Describe("Setup flow", func() {
	Describe("DiscoverPipeline", func() {
		It("collects local includes", func() {
			dir := GinkgoT().TempDir()
			root := filepath.Join(dir, ".gitlab-ci.yml")
			stagesPath := filepath.Join(dir, "gitlab-ci", "stages", "base.yml")

			err := os.MkdirAll(filepath.Dir(stagesPath), 0o755)
			Expect(err).NotTo(HaveOccurred())

			rootContent := `include:
  - local: gitlab-ci/stages/*.yml
stages:
  - build
  - test
`
			err = os.WriteFile(root, []byte(rootContent), 0o644)
			Expect(err).NotTo(HaveOccurred())

			includedContent := `stages:
  - deploy
`
			err = os.WriteFile(stagesPath, []byte(includedContent), 0o644)
			Expect(err).NotTo(HaveOccurred())

			result, err := DiscoverPipeline(root)
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Stages).To(Equal([]string{"build", "test", "deploy"}))
		})
	})

	Describe("ApplyConfigToRootYAML", func() {
		It("adds include, variables, and stages", func() {
			original := []byte(`stages:
  - build
include:
  - local: gitlab-ci/stages/*.yml
variables:
  GITLAB_CI_HELPER_TEMPLATE_PROJECT: platform/gitlab-ci-helper
  GITLAB_CI_HELPER_TEMPLATE_REF: v0.9.0
`)

			cfg := Config{
				Version: 1,
				Jobs: JobsConfig{
					AutoOpenMR: AutoOpenMRConfig{Enabled: true, Stage: "build", TriggerMode: "always_non_default"},
					CodexReview: CodexJobConfig{
						Enabled:      true,
						Stage:        "review",
						TriggerMode:  "manual_non_default",
						AllowFailure: true,
						Model:        "gpt-5.3-codex",
					},
					ReopenRelease: ReopenReleaseJob{Enabled: false},
				},
			}

			updated, err := ApplyConfigToRootYAML(original, cfg, []string{"review"})
			Expect(err).NotTo(HaveOccurred())

			var parsed map[string]any
			Expect(yaml.Unmarshal(updated, &parsed)).To(Succeed())

			stages := parsed["stages"].([]any)
			Expect(stages).To(HaveLen(2))
			Expect(stages[1]).To(Equal("review"))

			include := parsed["include"].([]any)
			Expect(include).To(HaveLen(2))
			foundLocalInclude := false
			for _, item := range include {
				entry, ok := item.(map[string]any)
				if !ok {
					continue
				}
				localValue, ok := entry["local"].(string)
				if !ok {
					continue
				}
				if localValue == LocalTemplatePath {
					foundLocalInclude = true
					break
				}
			}
			Expect(foundLocalInclude).To(BeTrue())

			variables := parsed["variables"].(map[string]any)
			Expect(variables["GITLAB_CI_HELPER_MR_TEMPLATE_PATH"]).To(Equal(LocalMRTemplatePath))
			Expect(variables[EnvCodexImage]).To(Equal(defaultCodexImage()))
			Expect(variables["GITLAB_CI_HELPER_CODEX_REVIEW_MODEL"]).To(Equal("gpt-5.3-codex"))
			Expect(variables).NotTo(HaveKey("GITLAB_CI_HELPER_TEMPLATE_PROJECT"))
			Expect(variables).NotTo(HaveKey("GITLAB_CI_HELPER_TEMPLATE_REF"))
		})

		It("uses MR description override path", func() {
			overridePath := ".gitlab/merge_request_templates/release.md"
			cfg := defaultConfig()
			cfg.Jobs.AutoOpenMR.MRDescriptionOverridePath = &overridePath

			updated, err := ApplyConfigToRootYAML([]byte("stages:\n  - build\n"), cfg, nil)
			Expect(err).NotTo(HaveOccurred())

			var parsed map[string]any
			Expect(yaml.Unmarshal(updated, &parsed)).To(Succeed())

			variables := parsed["variables"].(map[string]any)
			Expect(variables[EnvMRTemplatePath]).To(Equal(overridePath))
		})

		It("keeps existing local include while cleaning legacy include", func() {
			original := []byte(`include:
  - project: platform/gitlab-ci-helper
    ref: v0.9.0
    file: /templates/gitlab-ci-helper.yml
  - local: .gitlab-ci-helper/gitlab-ci-helper.yml
`)

			cfg := defaultConfig()
			updated, err := ApplyConfigToRootYAML(original, cfg, nil)
			Expect(err).NotTo(HaveOccurred())

			var parsed map[string]any
			Expect(yaml.Unmarshal(updated, &parsed)).To(Succeed())

			include := parsed["include"].([]any)
			Expect(include).To(HaveLen(1))

			entry := include[0].(map[string]any)
			Expect(entry["local"]).To(Equal(LocalTemplatePath))
		})
	})

	Describe("syncHelperAssets", func() {
		It("writes all mapped files", func() {
			dir := GinkgoT().TempDir()

			cfg := defaultConfig()
			cfg.Jobs.AutoOpenMR.Stage = "checks"
			cfg.Jobs.CodexReview.Stage = "review"

			Expect(syncHelperAssets(dir, cfg)).To(Succeed())

			for _, mapping := range helperAssetMappings() {
				targetPath := filepath.Join(dir, mapping.target)
				info, err := os.Stat(targetPath)
				Expect(err).NotTo(HaveOccurred())
				Expect(info.Size()).To(BeNumerically(">", 0))
			}
		})
	})

	Describe("PlanSetupChange", func() {
		It("renders concrete stages in helper template", func() {
			cfg := defaultConfig()
			cfg.Jobs.AutoOpenMR.Stage = "checks"
			cfg.Jobs.CodexReview.Stage = "build"

			planned, err := PlanSetupChange([]byte("stages:\n  - build\n"), nil, cfg, []string{"checks"})
			Expect(err).NotTo(HaveOccurred())

			var helperTemplate string
			for _, asset := range planned.Assets {
				if asset.RelativePath == LocalTemplatePath {
					helperTemplate = string(asset.Body)
					break
				}
			}
			Expect(helperTemplate).NotTo(BeEmpty())

			Expect(helperTemplate).NotTo(ContainSubstring(autoOpenMRStagePlaceholder))
			Expect(helperTemplate).NotTo(ContainSubstring(codexReviewStagePlaceholder))
			Expect(helperTemplate).NotTo(ContainSubstring(EnvAutoOpenMRStage))
			Expect(helperTemplate).NotTo(ContainSubstring(EnvCodexReviewStage))
			Expect(helperTemplate).To(ContainSubstring("stage: checks"))
			Expect(helperTemplate).To(ContainSubstring("stage: build"))
		})

		It("is idempotent after first apply", func() {
			cfg := defaultConfig()
			cfg.Jobs.AutoOpenMR.Stage = "build"
			cfg.Jobs.CodexReview.Stage = "build"

			initialRoot := []byte(`stages:
  - build
`)
			appliedRoot, err := ApplyConfigToRootYAML(initialRoot, cfg, nil)
			Expect(err).NotTo(HaveOccurred())

			cfgBody, err := marshalConfig(cfg)
			Expect(err).NotTo(HaveOccurred())

			planned, err := PlanSetupChange(appliedRoot, cfgBody, cfg, nil)
			Expect(err).NotTo(HaveOccurred())

			Expect(string(planned.RootPipeline.OriginalBody)).To(Equal(string(planned.RootPipeline.UpdatedBody)))
			Expect(string(planned.Config.OriginalBody)).To(Equal(string(planned.Config.UpdatedBody)))
		})
	})

	Describe("prompt helpers", func() {
		It("promptOption accepts value key", func() {
			reader := bufio.NewReader(strings.NewReader(TriggerManualNonDefault + "\n"))
			var out bytes.Buffer
			got, err := promptOption(reader, &out, "auto_open_mr trigger mode", TriggerAlwaysNonDefault, []option{
				{Value: TriggerAlwaysNonDefault, Label: "Always"},
				{Value: TriggerManualNonDefault, Label: "Manual"},
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(got).To(Equal(TriggerManualNonDefault))
		})

		It("promptStage normalizes case to known stage", func() {
			stageOrder := []string{"Build", "Review"}
			stageSet := map[string]struct{}{
				"Build":  {},
				"Review": {},
			}
			reader := bufio.NewReader(strings.NewReader("review\n"))
			var out bytes.Buffer

			stage, additions, err := promptStage(reader, &out, "codex_review", "Build", stageOrder, stageSet)
			Expect(err).NotTo(HaveOccurred())
			Expect(stage).To(Equal("Review"))
			Expect(additions).To(BeEmpty())
		})

		It("pickRecommendedStage prefers quality stage", func() {
			got := pickRecommendedStage("Checks", []string{"build", "test", "deploy"}, "auto_open_mr")
			Expect(got).To(Equal("test"))
		})
	})

	Describe("Run", func() {
		It("does not apply changes when user selects no", func() {
			dir := GinkgoT().TempDir()
			rootPath := filepath.Join(dir, rootPipelineFile)
			originalRoot := `stages:
  - build
`
			Expect(os.WriteFile(rootPath, []byte(originalRoot), 0o644)).To(Succeed())

			input := strings.Join([]string{
				"",
				"build",
				"",
				"",
				"",
				"build",
				"",
				"",
				"",
				"",
				"n",
			}, "\n") + "\n"

			var out bytes.Buffer
			Expect(Run(&out, strings.NewReader(input), dir)).To(Succeed())

			_, err := os.Stat(filepath.Join(dir, ConfigPath))
			Expect(os.IsNotExist(err)).To(BeTrue())

			gotRoot, err := os.ReadFile(rootPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(gotRoot)).To(Equal(originalRoot))
			Expect(out.String()).To(ContainSubstring("No files were changed."))
		})

		It("applies and writes root, config, and assets", func() {
			dir := GinkgoT().TempDir()
			rootPath := filepath.Join(dir, rootPipelineFile)
			Expect(os.WriteFile(rootPath, []byte("stages:\n  - build\n"), 0o644)).To(Succeed())

			input := strings.Join([]string{
				"",
				"build",
				"",
				"",
				"",
				"build",
				"",
				"",
				"",
				"",
				"y",
			}, "\n") + "\n"

			var out bytes.Buffer
			Expect(Run(&out, strings.NewReader(input), dir)).To(Succeed())

			rootBody, err := os.ReadFile(rootPath)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(rootBody)).To(ContainSubstring(LocalTemplatePath))
			Expect(string(rootBody)).To(ContainSubstring(EnvAutoOpenMREnabled))

			_, err = os.Stat(filepath.Join(dir, ConfigPath))
			Expect(err).NotTo(HaveOccurred())

			_, err = os.Stat(filepath.Join(dir, LocalTemplatesSubdir, "scripts", "auto_open_mr.sh"))
			Expect(err).NotTo(HaveOccurred())
		})
	})
})
