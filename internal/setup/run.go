package setup

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"path/filepath"
)

type setupState struct {
	OriginalRootPipeline []byte
	OriginalConfig       []byte
	ExistingConfig       *Config
	ExistingConfigErr    error
	Discovery            DiscoveryResult
	DiscoveryErr         error
}

// Run executes the interactive setup flow.
func Run(out io.Writer, in io.Reader, cwd string) error {
	state, err := loadSetupState(cwd)
	if err != nil {
		return err
	}
	if state.DiscoveryErr != nil {
		fmt.Fprintf(out, "Warning: pipeline discovery fallback mode: %v\n", state.DiscoveryErr)
	}
	if state.ExistingConfigErr != nil {
		fmt.Fprintf(out, "Warning: existing setup config ignored: %v\n", state.ExistingConfigErr)
	}

	reader := bufio.NewReader(in)
	cfg, stageAdditions, err := collectConfig(out, reader, state.ExistingConfig, state.Discovery)
	if err != nil {
		return err
	}

	planned, err := PlanSetupChange(state.OriginalRootPipeline, state.OriginalConfig, cfg, stageAdditions)
	if err != nil {
		return err
	}

	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Preview:")
	printDiff(out, planned.RootPipeline.RelativePath, string(planned.RootPipeline.OriginalBody), string(planned.RootPipeline.UpdatedBody))
	printDiff(out, planned.Config.RelativePath, string(planned.Config.OriginalBody), string(planned.Config.UpdatedBody))
	fmt.Fprintf(out, "Assets to sync: %d files under %s\n", len(planned.Assets), helperDir)

	apply, err := promptYesNo(reader, out, "Apply these changes", false)
	if err != nil {
		return err
	}
	if !apply {
		fmt.Fprintln(out, "No files were changed.")
		return nil
	}

	if err := ApplyPlannedChange(cwd, planned); err != nil {
		return err
	}

	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Applied changes.")
	fmt.Fprintln(out, "Set these project CI/CD variables in GitLab (Settings > CI/CD > Variables):")
	fmt.Fprintf(out, "- %s (masked, protected as needed)\n", EnvToken)
	fmt.Fprintf(out, "- %s (file variable, required only if codex_review is enabled)\n", EnvCodexAuth)
	fmt.Fprintf(out, "Commit %s to the repository so jobs and scripts are available in CI.\n", helperDir)
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "The wizard never stores secret values in repository files.")

	return nil
}

func loadSetupState(cwd string) (setupState, error) {
	rootPath := filepath.Join(cwd, rootPipelineFile)
	originalRoot, err := os.ReadFile(rootPath)
	if err != nil {
		return setupState{}, fmt.Errorf("read %s: %w", rootPipelineFile, err)
	}

	existingConfig, existingConfigErr := readExistingConfig(cwd)

	var originalConfig []byte
	cfgPath := filepath.Join(cwd, ConfigPath)
	originalConfig, err = os.ReadFile(cfgPath)
	if err != nil && !os.IsNotExist(err) {
		return setupState{}, fmt.Errorf("read %s: %w", ConfigPath, err)
	}

	discovery, discoveryErr := DiscoverPipeline(rootPath)
	if discoveryErr != nil {
		// Keep fallback behavior for partially-parsed projects.
		discovery = DiscoveryResult{}
	}

	return setupState{
		OriginalRootPipeline: originalRoot,
		OriginalConfig:       originalConfig,
		ExistingConfig:       existingConfig,
		ExistingConfigErr:    existingConfigErr,
		Discovery:            discovery,
		DiscoveryErr:         discoveryErr,
	}, nil
}
