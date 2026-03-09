package setup

import (
	"bufio"
	"bytes"
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
	ui := newSetupUI(out)

	state, err := loadSetupState(cwd)
	if err != nil {
		return err
	}
	ui.progress(1, 4, "Repository context loaded")
	if state.DiscoveryErr != nil {
		fmt.Fprintln(out, ui.warningLine(fmt.Sprintf("Pipeline discovery fallback mode: %v", state.DiscoveryErr)))
	}
	if state.ExistingConfigErr != nil {
		fmt.Fprintln(out, ui.warningLine(fmt.Sprintf("Existing setup config ignored: %v", state.ExistingConfigErr)))
	}

	reader := bufio.NewReader(in)
	cfg, stageAdditions, err := collectConfig(out, reader, state.ExistingConfig, state.Discovery)
	if err != nil {
		return err
	}
	ui.progress(2, 4, "Interactive configuration complete")

	planned, err := PlanSetupChange(state.OriginalRootPipeline, state.OriginalConfig, cfg, stageAdditions)
	if err != nil {
		return err
	}
	ui.progress(3, 4, "Change plan generated")

	printPlannedChangeOverview(out, planned)

	ui.section("Diff Preview", "Unified diff before writing any files.")
	printDiff(out, planned.RootPipeline.RelativePath, string(planned.RootPipeline.OriginalBody), string(planned.RootPipeline.UpdatedBody))
	printDiff(out, planned.Config.RelativePath, string(planned.Config.OriginalBody), string(planned.Config.UpdatedBody))
	fmt.Fprintln(out, ui.infoLine(fmt.Sprintf("Asset files to sync: %d (under %s)", len(planned.Assets), helperDir)))

	apply, err := promptYesNo(reader, out, "Apply these changes", false)
	if err != nil {
		return err
	}
	if !apply {
		fmt.Fprintln(out, ui.warningLine("No files were changed."))
		return nil
	}

	if err := ApplyPlannedChange(cwd, planned); err != nil {
		return err
	}
	ui.progress(4, 4, "Files written to repository")

	fmt.Fprintln(out, ui.successLine("Applied changes."))
	ui.printPanel("Next Steps", []string{
		"1. Set CI/CD variables in GitLab (Settings > CI/CD > Variables):",
		fmt.Sprintf("   • %s (masked, protected as needed)", EnvToken),
		fmt.Sprintf("   • %s (file variable, needed when codex_review is enabled)", EnvCodexAuth),
		fmt.Sprintf("2. Commit %s so jobs/scripts are available in CI.", helperDir),
		"3. Run a pipeline and validate helper jobs in your project context.",
	})
	fmt.Fprintln(out, ui.infoLine("The wizard never stores secret values in repository files."))

	return nil
}

func printPlannedChangeOverview(out io.Writer, planned PlannedSetupChange) {
	ui := newSetupUI(out)
	ui.section("Execution Plan", "Proposed writes before apply.")
	rows := [][]string{
		{statusLabel(describeChange(planned.RootPipeline)), planned.RootPipeline.RelativePath},
		{statusLabel(describeChange(planned.Config)), planned.Config.RelativePath},
		{fmt.Sprintf("sync %d", len(planned.Assets)), helperDir + "/..."},
	}
	ui.printTable([]string{"Action", "Target"}, rows)
}

func statusLabel(status string) string {
	switch status {
	case "create":
		return "create"
	case "keep  ":
		return "keep"
	default:
		return "update"
	}
}

func describeChange(change PlannedFileChange) string {
	switch {
	case len(change.OriginalBody) == 0 && len(change.UpdatedBody) > 0:
		return "create"
	case bytes.Equal(change.OriginalBody, change.UpdatedBody):
		return "keep  "
	default:
		return "update"
	}
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
