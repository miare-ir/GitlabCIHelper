package setup

import (
	"bufio"
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"gopkg.in/yaml.v3"
)

func TestDiscoverPipelineCollectsLocalIncludes(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	root := filepath.Join(dir, ".gitlab-ci.yml")
	stagesPath := filepath.Join(dir, "gitlab-ci", "stages", "base.yml")
	if err := os.MkdirAll(filepath.Dir(stagesPath), 0o755); err != nil {
		t.Fatalf("mkdir: %v", err)
	}

	rootContent := `include:
  - local: gitlab-ci/stages/*.yml
stages:
  - build
  - test
`
	if err := os.WriteFile(root, []byte(rootContent), 0o644); err != nil {
		t.Fatalf("write root: %v", err)
	}

	includedContent := `stages:
  - deploy
`
	if err := os.WriteFile(stagesPath, []byte(includedContent), 0o644); err != nil {
		t.Fatalf("write include: %v", err)
	}

	result, err := DiscoverPipeline(root)
	if err != nil {
		t.Fatalf("discover: %v", err)
	}

	if len(result.Stages) != 3 {
		t.Fatalf("expected 3 stages, got %d (%v)", len(result.Stages), result.Stages)
	}
	if result.Stages[0] != "build" || result.Stages[1] != "test" || result.Stages[2] != "deploy" {
		t.Fatalf("unexpected stage order: %v", result.Stages)
	}
}

func TestApplyConfigToRootYAMLAddsIncludeVariablesAndStages(t *testing.T) {
	t.Parallel()

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
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	var parsed map[string]any
	if err := yaml.Unmarshal(updated, &parsed); err != nil {
		t.Fatalf("unmarshal updated: %v", err)
	}

	stages := parsed["stages"].([]any)
	if len(stages) != 2 || stages[1].(string) != "review" {
		t.Fatalf("expected review stage appended, got %v", stages)
	}

	include := parsed["include"].([]any)
	if len(include) != 2 {
		t.Fatalf("expected include entries length 2, got %d", len(include))
	}
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
	if !foundLocalInclude {
		t.Fatalf("expected include to contain local helper template path %q, got %v", LocalTemplatePath, include)
	}

	variables := parsed["variables"].(map[string]any)
	if variables["GITLAB_CI_HELPER_MR_TEMPLATE_PATH"].(string) != LocalMRTemplatePath {
		t.Fatalf("mr template variable missing or unexpected: %v", variables)
	}
	if variables[EnvCodexImage].(string) != "git.miare.ir:5050/miare/images/codex:latest" {
		t.Fatalf("codex image variable missing or unexpected: %v", variables)
	}
	if variables["GITLAB_CI_HELPER_CODEX_REVIEW_MODEL"].(string) != "gpt-5.3-codex" {
		t.Fatalf("codex model variable missing: %v", variables)
	}
	if _, ok := variables["GITLAB_CI_HELPER_TEMPLATE_PROJECT"]; ok {
		t.Fatalf("expected legacy template project variable to be removed, got %v", variables)
	}
	if _, ok := variables["GITLAB_CI_HELPER_TEMPLATE_REF"]; ok {
		t.Fatalf("expected legacy template ref variable to be removed, got %v", variables)
	}
}

func TestApplyConfigToRootYAMLUsesMRDescriptionOverridePath(t *testing.T) {
	t.Parallel()

	overridePath := ".gitlab/merge_request_templates/release.md"
	cfg := defaultConfig()
	cfg.Jobs.AutoOpenMR.MRDescriptionOverridePath = &overridePath

	updated, err := ApplyConfigToRootYAML([]byte("stages:\n  - build\n"), cfg, nil)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	var parsed map[string]any
	if err := yaml.Unmarshal(updated, &parsed); err != nil {
		t.Fatalf("unmarshal updated: %v", err)
	}

	variables := parsed["variables"].(map[string]any)
	if variables[EnvMRTemplatePath].(string) != overridePath {
		t.Fatalf("expected %s=%q, got %v", EnvMRTemplatePath, overridePath, variables[EnvMRTemplatePath])
	}
}

func TestApplyConfigToRootYAMLKeepsExistingLocalInclude(t *testing.T) {
	t.Parallel()

	original := []byte(`include:
  - project: platform/gitlab-ci-helper
    ref: v0.9.0
    file: /templates/gitlab-ci-helper.yml
  - local: .gitlab-ci-helper/gitlab-ci-helper.yml
`)

	cfg := defaultConfig()

	updated, err := ApplyConfigToRootYAML(original, cfg, nil)
	if err != nil {
		t.Fatalf("apply: %v", err)
	}

	var parsed map[string]any
	if err := yaml.Unmarshal(updated, &parsed); err != nil {
		t.Fatalf("unmarshal: %v", err)
	}

	include := parsed["include"].([]any)
	if len(include) != 1 {
		t.Fatalf("expected single local include entry after legacy cleanup, got %d (%v)", len(include), include)
	}

	entry := include[0].(map[string]any)
	if entry["local"].(string) != LocalTemplatePath {
		t.Fatalf("expected local include to remain %q, got %v", LocalTemplatePath, entry["local"])
	}
}

func TestSyncHelperAssetsWritesFiles(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()

	cfg := defaultConfig()
	cfg.Jobs.AutoOpenMR.Stage = "checks"
	cfg.Jobs.CodexReview.Stage = "review"

	if err := syncHelperAssets(dir, cfg); err != nil {
		t.Fatalf("sync assets: %v", err)
	}

	for _, mapping := range helperAssetMappings() {
		targetPath := filepath.Join(dir, mapping.target)
		info, err := os.Stat(targetPath)
		if err != nil {
			t.Fatalf("expected synced file %s: %v", mapping.target, err)
		}
		if info.Size() == 0 {
			t.Fatalf("expected synced file %s to be non-empty", mapping.target)
		}
	}
}

func TestPlanSetupChangeRendersConcreteStagesInHelperTemplate(t *testing.T) {
	t.Parallel()

	cfg := defaultConfig()
	cfg.Jobs.AutoOpenMR.Stage = "checks"
	cfg.Jobs.CodexReview.Stage = "build"

	planned, err := PlanSetupChange([]byte("stages:\n  - build\n"), nil, cfg, []string{"checks"})
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	var helperTemplate string
	for _, asset := range planned.Assets {
		if asset.RelativePath == LocalTemplatePath {
			helperTemplate = string(asset.Body)
			break
		}
	}
	if helperTemplate == "" {
		t.Fatalf("expected planned assets to include %s", LocalTemplatePath)
	}

	if strings.Contains(helperTemplate, autoOpenMRStagePlaceholder) || strings.Contains(helperTemplate, codexReviewStagePlaceholder) {
		t.Fatalf("expected helper template placeholders to be rendered, got %q", helperTemplate)
	}
	if strings.Contains(helperTemplate, EnvAutoOpenMRStage) || strings.Contains(helperTemplate, EnvCodexReviewStage) {
		t.Fatalf("expected helper template to avoid env-based stage references, got %q", helperTemplate)
	}
	if !strings.Contains(helperTemplate, "stage: checks") {
		t.Fatalf("expected rendered auto_open_mr stage, got %q", helperTemplate)
	}
	if !strings.Contains(helperTemplate, "stage: build") {
		t.Fatalf("expected rendered codex_review stage, got %q", helperTemplate)
	}
}

func TestPlanSetupChangeIsIdempotentAfterFirstApply(t *testing.T) {
	t.Parallel()

	cfg := defaultConfig()
	cfg.Jobs.AutoOpenMR.Stage = "build"
	cfg.Jobs.CodexReview.Stage = "build"

	initialRoot := []byte(`stages:
  - build
`)
	appliedRoot, err := ApplyConfigToRootYAML(initialRoot, cfg, nil)
	if err != nil {
		t.Fatalf("first apply root: %v", err)
	}

	cfgBody, err := marshalConfig(cfg)
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}

	planned, err := PlanSetupChange(appliedRoot, cfgBody, cfg, nil)
	if err != nil {
		t.Fatalf("plan: %v", err)
	}

	if string(planned.RootPipeline.OriginalBody) != string(planned.RootPipeline.UpdatedBody) {
		t.Fatalf("expected root pipeline plan to be idempotent")
	}
	if string(planned.Config.OriginalBody) != string(planned.Config.UpdatedBody) {
		t.Fatalf("expected config plan to be idempotent")
	}
}

func TestPromptOptionAcceptsValueKey(t *testing.T) {
	t.Parallel()

	reader := bufio.NewReader(strings.NewReader(TriggerManualNonDefault + "\n"))
	var out bytes.Buffer
	got, err := promptOption(reader, &out, "auto_open_mr trigger mode", TriggerAlwaysNonDefault, []option{
		{Value: TriggerAlwaysNonDefault, Label: "Always"},
		{Value: TriggerManualNonDefault, Label: "Manual"},
	})
	if err != nil {
		t.Fatalf("promptOption: %v", err)
	}
	if got != TriggerManualNonDefault {
		t.Fatalf("expected trigger mode %q, got %q", TriggerManualNonDefault, got)
	}
}

func TestPromptStageNormalizesCaseToKnownStage(t *testing.T) {
	t.Parallel()

	stageOrder := []string{"Build", "Review"}
	stageSet := map[string]struct{}{
		"Build":  {},
		"Review": {},
	}
	reader := bufio.NewReader(strings.NewReader("review\n"))
	var out bytes.Buffer

	stage, additions, err := promptStage(reader, &out, "codex_review", "Build", stageOrder, stageSet)
	if err != nil {
		t.Fatalf("promptStage: %v", err)
	}
	if stage != "Review" {
		t.Fatalf("expected canonical stage %q, got %q", "Review", stage)
	}
	if len(additions) != 0 {
		t.Fatalf("expected no stage additions, got %v", additions)
	}
}

func TestPickRecommendedStagePrefersQualityStage(t *testing.T) {
	t.Parallel()

	got := pickRecommendedStage("Checks", []string{"build", "test", "deploy"}, "auto_open_mr")
	if got != "test" {
		t.Fatalf("expected recommended stage %q, got %q", "test", got)
	}
}

func TestRunWithSingleReaderNoApply(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	rootPath := filepath.Join(dir, rootPipelineFile)
	originalRoot := `stages:
  - build
`
	if err := os.WriteFile(rootPath, []byte(originalRoot), 0o644); err != nil {
		t.Fatalf("write root: %v", err)
	}

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
	if err := Run(&out, strings.NewReader(input), dir); err != nil {
		t.Fatalf("run: %v", err)
	}

	if _, err := os.Stat(filepath.Join(dir, ConfigPath)); !os.IsNotExist(err) {
		t.Fatalf("expected %s not to be created when apply=no", ConfigPath)
	}
	gotRoot, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatalf("read root: %v", err)
	}
	if string(gotRoot) != originalRoot {
		t.Fatalf("expected root file to remain unchanged")
	}
	if !strings.Contains(out.String(), "No files were changed.") {
		t.Fatalf("expected no-change message in output")
	}
}

func TestRunApplyWritesRootConfigAndAssets(t *testing.T) {
	t.Parallel()

	dir := t.TempDir()
	rootPath := filepath.Join(dir, rootPipelineFile)
	if err := os.WriteFile(rootPath, []byte("stages:\n  - build\n"), 0o644); err != nil {
		t.Fatalf("write root: %v", err)
	}

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
	if err := Run(&out, strings.NewReader(input), dir); err != nil {
		t.Fatalf("run: %v", err)
	}

	rootBody, err := os.ReadFile(rootPath)
	if err != nil {
		t.Fatalf("read root: %v", err)
	}
	if !strings.Contains(string(rootBody), LocalTemplatePath) {
		t.Fatalf("expected root pipeline to include %s", LocalTemplatePath)
	}
	if !strings.Contains(string(rootBody), EnvAutoOpenMREnabled) {
		t.Fatalf("expected root pipeline variables to include %s", EnvAutoOpenMREnabled)
	}

	if _, err := os.Stat(filepath.Join(dir, ConfigPath)); err != nil {
		t.Fatalf("expected config file to be written: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, LocalTemplatesSubdir, "scripts", "auto_open_mr.sh")); err != nil {
		t.Fatalf("expected helper asset to be written: %v", err)
	}
}
