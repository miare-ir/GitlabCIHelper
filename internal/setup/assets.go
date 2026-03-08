package setup

import (
	"fmt"
	"strings"

	templateassets "gitlab_ci_helper"
)

const (
	autoOpenMRStagePlaceholder  = "__GITLAB_CI_HELPER_AUTO_OPEN_MR_STAGE__"
	codexReviewStagePlaceholder = "__GITLAB_CI_HELPER_CODEX_REVIEW_STAGE__"
)

func helperAssetMappings() []helperAssetMapping {
	return []helperAssetMapping{
		{source: "templates/gitlab-ci-helper.yml", target: LocalTemplatePath},
		{source: "templates/mr_description.md", target: LocalMRTemplatePath},
		{source: "templates/scripts/auto_open_mr.sh", target: LocalTemplatesSubdir + "/scripts/auto_open_mr.sh"},
		{source: "templates/scripts/run_codex_review.sh", target: LocalTemplatesSubdir + "/scripts/run_codex_review.sh"},
		{source: "templates/scripts/codex_to_gitlab_discussions.sh", target: LocalTemplatesSubdir + "/scripts/codex_to_gitlab_discussions.sh"},
		{source: "templates/codex/review_prompt.md", target: LocalTemplatesSubdir + "/codex/review_prompt.md"},
		{source: "templates/codex/review_output_schema.json", target: LocalTemplatesSubdir + "/codex/review_output_schema.json"},
	}
}

func plannedHelperAssets(cfg Config) ([]PlannedAssetWrite, error) {
	assets := make([]PlannedAssetWrite, 0, len(helperAssetMappings()))
	for _, mapping := range helperAssetMappings() {
		body, err := templateassets.EmbeddedTemplates.ReadFile(mapping.source)
		if err != nil {
			return nil, fmt.Errorf("read embedded asset %s: %w", mapping.source, err)
		}
		if mapping.source == "templates/gitlab-ci-helper.yml" {
			body = renderTemplateStages(body, cfg)
		}
		assets = append(assets, PlannedAssetWrite{RelativePath: mapping.target, Body: body})
	}
	return assets, nil
}

func syncHelperAssets(cwd string, cfg Config) error {
	assets, err := plannedHelperAssets(cfg)
	if err != nil {
		return err
	}
	for _, asset := range assets {
		if err := writeRepoFileAtomic(cwd, asset.RelativePath, asset.Body, 0o644); err != nil {
			return err
		}
	}
	return nil
}

func renderTemplateStages(body []byte, cfg Config) []byte {
	updated := strings.ReplaceAll(string(body), autoOpenMRStagePlaceholder, cfg.Jobs.AutoOpenMR.Stage)
	updated = strings.ReplaceAll(updated, codexReviewStagePlaceholder, cfg.Jobs.CodexReview.Stage)
	return []byte(updated)
}
