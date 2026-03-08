package setup

import (
	"bytes"
	"fmt"
	"strings"

	"gopkg.in/yaml.v3"
)

// ApplyConfigToRootYAML updates include/variables/stages based on config.
func ApplyConfigToRootYAML(original []byte, cfg Config, stageAdditions []string) ([]byte, error) {
	var doc yaml.Node
	if err := yaml.Unmarshal(original, &doc); err != nil {
		return nil, fmt.Errorf("parse %s: %w", rootPipelineFile, err)
	}

	if len(doc.Content) == 0 {
		doc.Content = []*yaml.Node{{Kind: yaml.MappingNode, Tag: "!!map"}}
	}
	root := doc.Content[0]
	if root.Kind != yaml.MappingNode {
		return nil, fmt.Errorf("%s must have a top-level mapping", rootPipelineFile)
	}

	if err := upsertStages(root, stageAdditions); err != nil {
		return nil, err
	}
	if err := upsertLocalInclude(root, LocalTemplatePath); err != nil {
		return nil, err
	}
	if err := upsertVariables(root, cfg); err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	enc := yaml.NewEncoder(&buf)
	enc.SetIndent(2)
	if err := enc.Encode(&doc); err != nil {
		return nil, fmt.Errorf("encode %s: %w", rootPipelineFile, err)
	}
	if err := enc.Close(); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func upsertStages(root *yaml.Node, additions []string) error {
	if len(additions) == 0 {
		return nil
	}

	stages := mappingValue(root, "stages")
	if stages == nil {
		stages = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		setMappingValue(root, "stages", stages)
	}
	if stages.Kind != yaml.SequenceNode {
		return fmt.Errorf("stages key exists but is not a sequence")
	}

	existing := map[string]struct{}{}
	for _, stage := range stages.Content {
		existing[strings.TrimSpace(stage.Value)] = struct{}{}
	}
	for _, addition := range additions {
		if _, ok := existing[addition]; ok {
			continue
		}
		existing[addition] = struct{}{}
		stages.Content = append(stages.Content, scalarNode(addition))
	}
	return nil
}

func upsertLocalInclude(root *yaml.Node, includePath string) error {
	includePath = strings.TrimSpace(includePath)
	if includePath == "" {
		return fmt.Errorf("include path is required")
	}

	include := mappingValue(root, "include")
	if include == nil {
		include = &yaml.Node{Kind: yaml.SequenceNode, Tag: "!!seq"}
		setMappingValue(root, "include", include)
	}

	if include.Kind != yaml.SequenceNode {
		copyNode := *include
		include.Kind = yaml.SequenceNode
		include.Tag = "!!seq"
		include.Content = []*yaml.Node{&copyNode}
		include.Value = ""
	}

	filtered := make([]*yaml.Node, 0, len(include.Content))
	hasLocalInclude := false
	for _, item := range include.Content {
		switch item.Kind {
		case yaml.ScalarNode:
			if strings.TrimSpace(item.Value) == includePath {
				hasLocalInclude = true
			}
		case yaml.MappingNode:
			localNode := mappingValue(item, "local")
			if localNode == nil {
				if isLegacyHelperProjectInclude(item) {
					continue
				}
				filtered = append(filtered, item)
				continue
			}
			if strings.TrimSpace(localNode.Value) == includePath {
				hasLocalInclude = true
			}
		}
		filtered = append(filtered, item)
	}
	include.Content = filtered

	if !hasLocalInclude {
		entry := &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		setMappingScalar(entry, "local", includePath)
		include.Content = append(include.Content, entry)
	}
	return nil
}

func isLegacyHelperProjectInclude(node *yaml.Node) bool {
	if node == nil || node.Kind != yaml.MappingNode {
		return false
	}
	projectNode := mappingValue(node, "project")
	fileNode := mappingValue(node, "file")
	if projectNode == nil || fileNode == nil {
		return false
	}
	if strings.TrimSpace(projectNode.Value) == "" {
		return false
	}
	fileValue := strings.TrimSpace(fileNode.Value)
	return fileValue == "/templates/gitlab-ci-helper.yml" || strings.HasSuffix(fileValue, "/gitlab-ci-helper.yml")
}

func upsertVariables(root *yaml.Node, cfg Config) error {
	vars := mappingValue(root, "variables")
	if vars == nil {
		vars = &yaml.Node{Kind: yaml.MappingNode, Tag: "!!map"}
		setMappingValue(root, "variables", vars)
	}
	if vars.Kind != yaml.MappingNode {
		return fmt.Errorf("variables key exists but is not a mapping")
	}

	setMappingScalar(vars, EnvConfigPath, ConfigPath)
	setMappingScalar(vars, EnvMRTemplatePath, resolveMRTemplatePath(cfg))

	setMappingScalar(vars, EnvAutoOpenMREnabled, boolString(cfg.Jobs.AutoOpenMR.Enabled))
	setMappingScalar(vars, EnvAutoOpenMRStage, cfg.Jobs.AutoOpenMR.Stage)
	setMappingScalar(vars, EnvAutoOpenMRTriggerMode, cfg.Jobs.AutoOpenMR.TriggerMode)

	setMappingScalar(vars, EnvCodexReviewEnabled, boolString(cfg.Jobs.CodexReview.Enabled))
	setMappingScalar(vars, EnvCodexReviewStage, cfg.Jobs.CodexReview.Stage)
	setMappingScalar(vars, EnvCodexReviewTriggerMode, cfg.Jobs.CodexReview.TriggerMode)
	setMappingScalar(vars, EnvCodexReviewAllowFailure, boolString(cfg.Jobs.CodexReview.AllowFailure))
	setMappingScalar(vars, EnvCodexReviewModel, cfg.Jobs.CodexReview.Model)
	setMappingScalar(vars, EnvCodexImage, "git.miare.ir:5050/miare/images/codex:latest")
	setMappingScalar(vars, EnvCodexPromptPath, derefOrEmpty(cfg.Jobs.CodexReview.PromptOverridePath))
	setMappingScalar(vars, EnvCodexSchemaPath, derefOrEmpty(cfg.Jobs.CodexReview.SchemaOverridePath))

	// Cleanup legacy remote-template variables from pre-standalone setups.
	removeMappingKey(vars, LegacyEnvTemplateProject)
	removeMappingKey(vars, LegacyEnvTemplateRef)
	return nil
}

func resolveMRTemplatePath(cfg Config) string {
	path := derefOrEmpty(cfg.Jobs.AutoOpenMR.MRDescriptionOverridePath)
	if path == "" {
		return LocalMRTemplatePath
	}
	return path
}
