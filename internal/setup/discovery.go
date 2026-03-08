package setup

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"gopkg.in/yaml.v3"
)

// DiscoverPipeline inspects .gitlab-ci.yml and local include chains for stage names.
func DiscoverPipeline(rootPath string) (DiscoveryResult, error) {
	seen := map[string]struct{}{}
	stageSet := map[string]struct{}{}
	var stages []string

	var walk func(string) error
	walk = func(path string) error {
		absPath, err := filepath.Abs(path)
		if err != nil {
			return err
		}
		if _, ok := seen[absPath]; ok {
			return nil
		}
		seen[absPath] = struct{}{}

		body, err := os.ReadFile(absPath)
		if err != nil {
			return err
		}

		var doc yaml.Node
		if err := yaml.Unmarshal(body, &doc); err != nil {
			return fmt.Errorf("parse %s: %w", absPath, err)
		}
		if len(doc.Content) == 0 || doc.Content[0].Kind != yaml.MappingNode {
			return nil
		}
		root := doc.Content[0]

		if stagesNode := mappingValue(root, "stages"); stagesNode != nil && stagesNode.Kind == yaml.SequenceNode {
			for _, entry := range stagesNode.Content {
				stage := strings.TrimSpace(entry.Value)
				if stage == "" {
					continue
				}
				if _, ok := stageSet[stage]; ok {
					continue
				}
				stageSet[stage] = struct{}{}
				stages = append(stages, stage)
			}
		}

		includeNode := mappingValue(root, "include")
		locals := collectLocalIncludes(includeNode)
		baseDir := filepath.Dir(absPath)
		for _, local := range locals {
			if strings.Contains(local, "$") {
				continue
			}

			candidate := filepath.Join(baseDir, local)
			if hasGlob(local) {
				matches, err := filepath.Glob(candidate)
				if err != nil {
					continue
				}
				sort.Strings(matches)
				for _, m := range matches {
					if err := walk(m); err != nil {
						return err
					}
				}
				continue
			}
			if err := walk(candidate); err != nil {
				if errors.Is(err, os.ErrNotExist) {
					continue
				}
				return err
			}
		}
		return nil
	}

	if err := walk(rootPath); err != nil {
		return DiscoveryResult{}, err
	}

	return DiscoveryResult{Stages: stages}, nil
}

func collectLocalIncludes(includeNode *yaml.Node) []string {
	if includeNode == nil {
		return nil
	}

	var values []string
	collect := func(n *yaml.Node) {
		if n == nil {
			return
		}
		if n.Kind == yaml.MappingNode {
			for i := 0; i+1 < len(n.Content); i += 2 {
				key := n.Content[i]
				value := n.Content[i+1]
				if key.Value == "local" && value.Kind == yaml.ScalarNode {
					values = append(values, strings.TrimSpace(value.Value))
				}
			}
		}
	}

	switch includeNode.Kind {
	case yaml.ScalarNode:
		v := strings.TrimSpace(includeNode.Value)
		if v != "" {
			values = append(values, v)
		}
	case yaml.MappingNode:
		collect(includeNode)
	case yaml.SequenceNode:
		for _, item := range includeNode.Content {
			switch item.Kind {
			case yaml.ScalarNode:
				v := strings.TrimSpace(item.Value)
				if v != "" {
					values = append(values, v)
				}
			case yaml.MappingNode:
				collect(item)
			}
		}
	}

	return values
}

func hasGlob(value string) bool {
	return strings.ContainsAny(value, "*?[]")
}
