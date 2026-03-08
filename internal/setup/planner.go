package setup

import "fmt"

// PlanSetupChange computes all setup outputs without writing files.
func PlanSetupChange(originalRoot, originalConfig []byte, cfg Config, stageAdditions []string) (PlannedSetupChange, error) {
	updatedRoot, err := ApplyConfigToRootYAML(originalRoot, cfg, stageAdditions)
	if err != nil {
		return PlannedSetupChange{}, err
	}

	updatedCfg, err := marshalConfig(cfg)
	if err != nil {
		return PlannedSetupChange{}, fmt.Errorf("marshal config: %w", err)
	}

	assets, err := plannedHelperAssets(cfg)
	if err != nil {
		return PlannedSetupChange{}, err
	}

	return PlannedSetupChange{
		RootPipeline: PlannedFileChange{
			RelativePath: rootPipelineFile,
			OriginalBody: originalRoot,
			UpdatedBody:  updatedRoot,
		},
		Config: PlannedFileChange{
			RelativePath: ConfigPath,
			OriginalBody: originalConfig,
			UpdatedBody:  updatedCfg,
		},
		Assets: assets,
	}, nil
}
