package setup

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type option struct {
	Value string
	Label string
}

func collectConfig(out io.Writer, reader *bufio.Reader, existing *Config, discovery DiscoveryResult) (Config, []string, error) {
	cfg := defaultConfig()
	if existing != nil {
		cfg = *existing
	}

	stageOrder := append([]string{}, discovery.Stages...)
	stageSet := map[string]struct{}{}
	for _, stage := range stageOrder {
		stageSet[stage] = struct{}{}
	}

	fmt.Fprintln(out, "GitLab CI Helper setup")
	fmt.Fprintln(out, "")

	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Configure job: auto_open_mr")
	fmt.Fprintln(out, "  Automatically creates a merge request for the current branch.")
	autoEnabled, err := promptYesNo(reader, out, "Enable auto_open_mr", cfg.Jobs.AutoOpenMR.Enabled)
	if err != nil {
		return Config{}, nil, err
	}
	cfg.Jobs.AutoOpenMR.Enabled = autoEnabled

	autoStage, additions, err := promptStage(reader, out, "auto_open_mr", cfg.Jobs.AutoOpenMR.Stage, stageOrder, stageSet)
	if err != nil {
		return Config{}, nil, err
	}
	cfg.Jobs.AutoOpenMR.Stage = autoStage
	for _, stage := range additions {
		stageSet[stage] = struct{}{}
		stageOrder = append(stageOrder, stage)
	}

	fmt.Fprintln(out, "  Controls when the auto_open_mr job is triggered.")
	autoModeOptions := []option{
		{Value: TriggerAlwaysNonDefault, Label: "Always on non-default branches (current method)"},
		{Value: TriggerManualNonDefault, Label: "Manual on non-default branches"},
		{Value: TriggerManualAnyBranch, Label: "Manual on any branch"},
	}
	autoMode, err := promptOption(reader, out, "auto_open_mr trigger mode", cfg.Jobs.AutoOpenMR.TriggerMode, autoModeOptions)
	if err != nil {
		return Config{}, nil, err
	}
	cfg.Jobs.AutoOpenMR.TriggerMode = autoMode

	fmt.Fprintln(out, "  Custom MR description template file path, relative to repo root. Leave blank for built-in.")
	mrDescriptionPathDefault := derefOrEmpty(cfg.Jobs.AutoOpenMR.MRDescriptionOverridePath)
	mrDescriptionPath, err := promptString(reader, out, "MR description override path (optional)", mrDescriptionPathDefault, false)
	if err != nil {
		return Config{}, nil, err
	}
	cfg.Jobs.AutoOpenMR.MRDescriptionOverridePath = nilIfEmpty(mrDescriptionPath)

	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Configure job: codex_review")
	fmt.Fprintln(out, "  AI-powered code review using Codex (requires GITLAB_CI_HELPER_CODEX_AUTH variable).")
	codexEnabled, err := promptYesNo(reader, out, "Enable codex_review", cfg.Jobs.CodexReview.Enabled)
	if err != nil {
		return Config{}, nil, err
	}
	cfg.Jobs.CodexReview.Enabled = codexEnabled

	codexStage, codexAdditions, err := promptStage(reader, out, "codex_review", cfg.Jobs.CodexReview.Stage, stageOrder, stageSet)
	if err != nil {
		return Config{}, nil, err
	}
	cfg.Jobs.CodexReview.Stage = codexStage
	for _, stage := range codexAdditions {
		if _, ok := stageSet[stage]; ok {
			continue
		}
		stageSet[stage] = struct{}{}
		stageOrder = append(stageOrder, stage)
	}

	fmt.Fprintln(out, "  Controls when the codex_review job is triggered.")
	codexModeOptions := []option{
		{Value: TriggerManualNonDefault, Label: "Manual on non-default branches"},
		{Value: TriggerAlwaysNonDefault, Label: "Always on non-default branches"},
		{Value: TriggerManualMREvent, Label: "Manual on merge_request_event pipelines"},
		{Value: TriggerAlwaysMREvent, Label: "Always on merge_request_event pipelines"},
	}
	codexMode, err := promptOption(reader, out, "codex_review trigger mode", cfg.Jobs.CodexReview.TriggerMode, codexModeOptions)
	if err != nil {
		return Config{}, nil, err
	}
	cfg.Jobs.CodexReview.TriggerMode = codexMode

	cfg.Jobs.CodexReview.AllowFailure = true
	fmt.Fprintln(out, "  AI model identifier passed to the review script (e.g. gpt-5.3-codex).")
	model, err := promptString(reader, out, "Codex model", cfg.Jobs.CodexReview.Model, true)
	if err != nil {
		return Config{}, nil, err
	}
	cfg.Jobs.CodexReview.Model = model

	fmt.Fprintln(out, "  Custom prompt template file path, relative to repo root. Leave blank for built-in.")
	promptPathDefault := derefOrEmpty(cfg.Jobs.CodexReview.PromptOverridePath)
	promptPath, err := promptString(reader, out, "Prompt override path (optional)", promptPathDefault, false)
	if err != nil {
		return Config{}, nil, err
	}
	cfg.Jobs.CodexReview.PromptOverridePath = nilIfEmpty(promptPath)

	fmt.Fprintln(out, "  Custom JSON schema for review output, relative to repo root. Leave blank for built-in.")
	schemaPathDefault := derefOrEmpty(cfg.Jobs.CodexReview.SchemaOverridePath)
	schemaPath, err := promptString(reader, out, "Schema override path (optional)", schemaPathDefault, false)
	if err != nil {
		return Config{}, nil, err
	}
	cfg.Jobs.CodexReview.SchemaOverridePath = nilIfEmpty(schemaPath)

	cfg.Jobs.ReopenRelease.Enabled = false
	cfg.Version = 1

	var stageAdditions []string
	for _, stage := range stageOrder {
		if !contains(discovery.Stages, stage) {
			stageAdditions = append(stageAdditions, stage)
		}
	}

	return cfg, stageAdditions, nil
}

func defaultConfig() Config {
	return Config{
		Version: 1,
		Jobs: JobsConfig{
			AutoOpenMR: AutoOpenMRConfig{
				Enabled:     true,
				Stage:       "Checks",
				TriggerMode: TriggerAlwaysNonDefault,
			},
			CodexReview: CodexJobConfig{
				Enabled:      true,
				Stage:        "Checks",
				TriggerMode:  TriggerManualNonDefault,
				AllowFailure: true,
				Model:        "gpt-5.3-codex",
			},
			ReopenRelease: ReopenReleaseJob{Enabled: false},
		},
	}
}

func promptStage(reader *bufio.Reader, out io.Writer, jobName string, current string, stageOrder []string, stageSet map[string]struct{}) (string, []string, error) {
	for {
		fmt.Fprintf(out, "Available stages for %s:\n", jobName)
		for idx, stage := range stageOrder {
			fmt.Fprintf(out, "  %d. %s\n", idx+1, stage)
		}
		fmt.Fprintf(out, "Choose stage number or enter custom stage")
		if current != "" {
			fmt.Fprintf(out, " [%s]", current)
		}
		fmt.Fprint(out, ": ")

		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", nil, err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			line = current
		}
		if line == "" && len(stageOrder) > 0 {
			line = stageOrder[0]
		}
		if line == "" {
			return "", nil, fmt.Errorf("stage is required")
		}

		if idx, convErr := strconv.Atoi(line); convErr == nil {
			if idx < 1 || idx > len(stageOrder) {
				fmt.Fprintln(out, "Invalid stage number.")
				continue
			}
			return stageOrder[idx-1], nil, nil
		}

		if _, ok := stageSet[line]; ok {
			return line, nil, nil
		}

		add, err := promptYesNo(reader, out, fmt.Sprintf("Stage '%s' does not exist. Add it", line), true)
		if err != nil {
			return "", nil, err
		}
		if !add {
			continue
		}
		return line, []string{line}, nil
	}
}

func promptOption(reader *bufio.Reader, out io.Writer, label string, current string, options []option) (string, error) {
	if len(options) == 0 {
		return "", fmt.Errorf("no options provided")
	}
	currentIdx := 0
	for idx, opt := range options {
		if opt.Value == current {
			currentIdx = idx
			break
		}
	}

	for {
		fmt.Fprintf(out, "%s:\n", label)
		for idx, opt := range options {
			fmt.Fprintf(out, "  %d. %s\n", idx+1, opt.Label)
		}
		fmt.Fprintf(out, "Select option [%d]: ", currentIdx+1)

		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			return options[currentIdx].Value, nil
		}

		idx, convErr := strconv.Atoi(line)
		if convErr != nil || idx < 1 || idx > len(options) {
			fmt.Fprintln(out, "Invalid option.")
			continue
		}
		return options[idx-1].Value, nil
	}
}

func promptString(reader *bufio.Reader, out io.Writer, label string, defaultValue string, required bool) (string, error) {
	for {
		fmt.Fprint(out, label)
		if defaultValue != "" {
			fmt.Fprintf(out, " [%s]", defaultValue)
		}
		fmt.Fprint(out, ": ")

		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			line = defaultValue
		}
		if required && strings.TrimSpace(line) == "" {
			fmt.Fprintln(out, "This value is required.")
			continue
		}
		return line, nil
	}
}

func promptYesNo(reader *bufio.Reader, out io.Writer, label string, defaultYes bool) (bool, error) {
	defaultLabel := "y/N"
	if defaultYes {
		defaultLabel = "Y/n"
	}
	for {
		fmt.Fprintf(out, "%s [%s]: ", label, defaultLabel)
		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return false, err
		}
		line = strings.ToLower(strings.TrimSpace(line))
		if line == "" {
			return defaultYes, nil
		}
		switch line {
		case "y", "yes":
			return true, nil
		case "n", "no":
			return false, nil
		default:
			fmt.Fprintln(out, "Please answer yes or no.")
		}
	}
}
