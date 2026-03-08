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
	hasExistingConfig := false
	if existing != nil {
		cfg = *existing
		hasExistingConfig = true
	}

	stageOrder := append([]string{}, discovery.Stages...)
	stageSet := map[string]struct{}{}
	for _, stage := range stageOrder {
		stageSet[stage] = struct{}{}
	}

	printWizardIntro(out, hasExistingConfig, discovery)

	printJobSection(out, 1, 2, "auto_open_mr", "Automatically creates a merge request for the current branch.")
	autoEnabled, err := promptYesNo(reader, out, "Enable auto_open_mr", cfg.Jobs.AutoOpenMR.Enabled)
	if err != nil {
		return Config{}, nil, err
	}
	cfg.Jobs.AutoOpenMR.Enabled = autoEnabled

	autoStageDefault := pickRecommendedStage(cfg.Jobs.AutoOpenMR.Stage, stageOrder, "auto_open_mr")
	autoStage, additions, err := promptStage(reader, out, "auto_open_mr", autoStageDefault, stageOrder, stageSet)
	if err != nil {
		return Config{}, nil, err
	}
	cfg.Jobs.AutoOpenMR.Stage = autoStage
	for _, stage := range additions {
		stageSet[stage] = struct{}{}
		stageOrder = append(stageOrder, stage)
	}

	fmt.Fprintln(out, "Choose when auto_open_mr should run.")
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

	fmt.Fprintln(out, "Custom MR description template path (repo-relative). Leave blank for built-in template.")
	mrDescriptionPathDefault := derefOrEmpty(cfg.Jobs.AutoOpenMR.MRDescriptionOverridePath)
	mrDescriptionPath, err := promptString(reader, out, "MR description override path (optional)", mrDescriptionPathDefault, false)
	if err != nil {
		return Config{}, nil, err
	}
	cfg.Jobs.AutoOpenMR.MRDescriptionOverridePath = nilIfEmpty(mrDescriptionPath)

	printJobSection(out, 2, 2, "codex_review", "AI-powered code review via Codex. Requires GITLAB_CI_HELPER_CODEX_AUTH.")
	codexEnabled, err := promptYesNo(reader, out, "Enable codex_review", cfg.Jobs.CodexReview.Enabled)
	if err != nil {
		return Config{}, nil, err
	}
	cfg.Jobs.CodexReview.Enabled = codexEnabled

	codexStageDefault := pickRecommendedStage(cfg.Jobs.CodexReview.Stage, stageOrder, "codex_review")
	codexStage, codexAdditions, err := promptStage(reader, out, "codex_review", codexStageDefault, stageOrder, stageSet)
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

	fmt.Fprintln(out, "Choose when codex_review should run.")
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
	fmt.Fprintln(out, "AI model identifier passed to the review script (for example: gpt-5.3-codex).")
	model, err := promptString(reader, out, "Codex model", cfg.Jobs.CodexReview.Model, true)
	if err != nil {
		return Config{}, nil, err
	}
	cfg.Jobs.CodexReview.Model = model

	fmt.Fprintln(out, "Custom prompt template path (repo-relative). Leave blank for built-in template.")
	promptPathDefault := derefOrEmpty(cfg.Jobs.CodexReview.PromptOverridePath)
	promptPath, err := promptString(reader, out, "Prompt override path (optional)", promptPathDefault, false)
	if err != nil {
		return Config{}, nil, err
	}
	cfg.Jobs.CodexReview.PromptOverridePath = nilIfEmpty(promptPath)

	fmt.Fprintln(out, "Custom JSON schema path (repo-relative). Leave blank for built-in schema.")
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

	printConfigSummary(out, cfg, stageAdditions)

	return cfg, stageAdditions, nil
}

func printWizardIntro(out io.Writer, hasExistingConfig bool, discovery DiscoveryResult) {
	fmt.Fprintln(out, "GitLab CI Helper Setup Wizard")
	fmt.Fprintln(out, "=============================")
	if hasExistingConfig {
		fmt.Fprintln(out, "Detected existing config. Using it as baseline defaults.")
	} else {
		fmt.Fprintln(out, "No existing config detected. Starting from smart defaults.")
	}
	if len(discovery.Stages) == 0 {
		fmt.Fprintln(out, "Stage discovery: no stages detected in include chain. You can still define new ones.")
	} else {
		fmt.Fprintf(out, "Stage discovery: %s\n", summarizeStages(discovery.Stages))
	}
	fmt.Fprintln(out, "Tip: press Enter to accept defaults shown in brackets.")
}

func printJobSection(out io.Writer, step int, total int, name string, description string) {
	header := fmt.Sprintf("[%d/%d] Configure job: %s", step, total, name)
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, header)
	fmt.Fprintln(out, strings.Repeat("-", len(header)))
	fmt.Fprintln(out, description)
}

func printConfigSummary(out io.Writer, cfg Config, stageAdditions []string) {
	fmt.Fprintln(out, "")
	fmt.Fprintln(out, "Configuration summary:")
	fmt.Fprintf(out, "  auto_open_mr: enabled=%t, stage=%s, trigger=%s, mr_template=%s\n",
		cfg.Jobs.AutoOpenMR.Enabled,
		cfg.Jobs.AutoOpenMR.Stage,
		cfg.Jobs.AutoOpenMR.TriggerMode,
		formatOptionalPath(cfg.Jobs.AutoOpenMR.MRDescriptionOverridePath),
	)
	fmt.Fprintf(out, "  codex_review: enabled=%t, stage=%s, trigger=%s, model=%s, prompt=%s, schema=%s\n",
		cfg.Jobs.CodexReview.Enabled,
		cfg.Jobs.CodexReview.Stage,
		cfg.Jobs.CodexReview.TriggerMode,
		cfg.Jobs.CodexReview.Model,
		formatOptionalPath(cfg.Jobs.CodexReview.PromptOverridePath),
		formatOptionalPath(cfg.Jobs.CodexReview.SchemaOverridePath),
	)
	if len(stageAdditions) == 0 {
		fmt.Fprintln(out, "  stages to append: none")
		return
	}
	fmt.Fprintf(out, "  stages to append: %s\n", strings.Join(stageAdditions, ", "))
}

func summarizeStages(stages []string) string {
	if len(stages) <= 6 {
		return strings.Join(stages, ", ")
	}
	return fmt.Sprintf("%s, ... (%d total)", strings.Join(stages[:6], ", "), len(stages))
}

func formatOptionalPath(value *string) string {
	if value == nil {
		return "(built-in)"
	}
	trimmed := strings.TrimSpace(*value)
	if trimmed == "" {
		return "(built-in)"
	}
	return trimmed
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

func pickRecommendedStage(current string, stageOrder []string, jobName string) string {
	trimmedCurrent := strings.TrimSpace(current)
	if normalized, ok := normalizeStageName(trimmedCurrent, stageOrder); ok {
		return normalized
	}
	if len(stageOrder) == 0 {
		return trimmedCurrent
	}
	for _, keyword := range preferredStageKeywords(jobName) {
		for _, stage := range stageOrder {
			if strings.Contains(strings.ToLower(stage), keyword) {
				return stage
			}
		}
	}
	return stageOrder[0]
}

func preferredStageKeywords(jobName string) []string {
	switch jobName {
	case "codex_review":
		return []string{"review", "check", "test", "verify", "lint"}
	default:
		return []string{"check", "test", "verify", "lint"}
	}
}

func promptStage(reader *bufio.Reader, out io.Writer, jobName string, current string, stageOrder []string, stageSet map[string]struct{}) (string, []string, error) {
	current = strings.TrimSpace(current)
	if current == "" && len(stageOrder) > 0 {
		current = stageOrder[0]
	}
	for {
		fmt.Fprintf(out, "Available stages for %s:\n", jobName)
		if len(stageOrder) == 0 {
			fmt.Fprintln(out, "  (none detected yet)")
		}
		for idx, stage := range stageOrder {
			marker := " "
			if stage == current {
				marker = "*"
			}
			fmt.Fprintf(out, "  %d. [%s] %s\n", idx+1, marker, stage)
		}
		fmt.Fprintf(out, "Choose stage number or name")
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

		if normalized, ok := normalizeStageName(line, stageOrder); ok {
			return normalized, nil, nil
		}

		if _, ok := stageSet[line]; ok {
			return line, nil, nil
		}

		add, err := promptYesNo(reader, out, fmt.Sprintf("Stage %q does not exist. Add it to stages", line), true)
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
			marker := " "
			if idx == currentIdx {
				marker = "*"
			}
			fmt.Fprintf(out, "  %d. [%s] %s (%s)\n", idx+1, marker, opt.Label, opt.Value)
		}
		fmt.Fprintf(out, "Select option (number or key) [%d]: ", currentIdx+1)

		line, err := reader.ReadString('\n')
		if err != nil && !errors.Is(err, io.EOF) {
			return "", err
		}
		line = strings.TrimSpace(line)
		if line == "" {
			return options[currentIdx].Value, nil
		}

		idx, convErr := strconv.Atoi(line)
		if convErr == nil {
			if idx < 1 || idx > len(options) {
				fmt.Fprintln(out, "Invalid option number.")
				continue
			}
			return options[idx-1].Value, nil
		}
		if value, ok := resolveOptionValue(line, options); ok {
			return value, nil
		}
		fmt.Fprintln(out, "Invalid option. Use the number or key shown in parentheses.")
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
		case "y", "yes", "1", "true":
			return true, nil
		case "n", "no", "0", "false":
			return false, nil
		default:
			fmt.Fprintln(out, "Please answer yes or no.")
		}
	}
}

func normalizeStageName(value string, stageOrder []string) (string, bool) {
	for _, stage := range stageOrder {
		if strings.EqualFold(stage, value) {
			return stage, true
		}
	}
	return "", false
}

func resolveOptionValue(value string, options []option) (string, bool) {
	for _, opt := range options {
		if strings.EqualFold(value, opt.Value) {
			return opt.Value, true
		}
	}
	return "", false
}
