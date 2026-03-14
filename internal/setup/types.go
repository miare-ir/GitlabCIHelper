package setup

// Config is the persistent wizard configuration stored in .gitlab-ci-helper/config.yml.
type Config struct {
	Version int        `yaml:"version"`
	Jobs    JobsConfig `yaml:"jobs"`
}

// JobsConfig controls all helper jobs.
type JobsConfig struct {
	AutoOpenMR    AutoOpenMRConfig `yaml:"auto_open_mr"`
	CodexReview   CodexJobConfig   `yaml:"codex_review"`
	ReopenRelease ReopenReleaseJob `yaml:"reopen_release"`
}

// AutoOpenMRConfig controls merge-request creation behavior.
type AutoOpenMRConfig struct {
	Enabled        bool   `yaml:"enabled"`
	Stage          string `yaml:"stage"`
	TriggerMode    string `yaml:"trigger_mode"`
	MRTemplatePath string `yaml:"mr_template_path,omitempty"`
}

// BaseJobConfig controls generic job settings.
type BaseJobConfig struct {
	Enabled     bool   `yaml:"enabled"`
	Stage       string `yaml:"stage"`
	TriggerMode string `yaml:"trigger_mode"`
}

// CodexJobConfig controls Codex review behavior.
type CodexJobConfig struct {
	Enabled            bool   `yaml:"enabled"`
	Stage              string `yaml:"stage"`
	TriggerMode        string `yaml:"trigger_mode"`
	AllowFailure       bool   `yaml:"allow_failure"`
	Model              string `yaml:"model"`
	Image              string `yaml:"image,omitempty"`
	PromptOverridePath string `yaml:"prompt_override_path,omitempty"`
	SchemaOverridePath string `yaml:"schema_override_path,omitempty"`
}

// ReopenReleaseJob is intentionally a disabled placeholder for v2.
type ReopenReleaseJob struct {
	Enabled bool `yaml:"enabled"`
}

// DiscoveryResult describes discovered pipeline details.
type DiscoveryResult struct {
	Stages []string
}

type helperAssetMapping struct {
	source string
	target string
}

// PlannedFileChange contains the before/after states of a single file.
type PlannedFileChange struct {
	RelativePath string
	OriginalBody []byte
	UpdatedBody  []byte
}

// PlannedAssetWrite represents an embedded helper asset to write.
type PlannedAssetWrite struct {
	RelativePath string
	Body         []byte
}

// PlannedSetupChange is a pure representation of all setup writes.
type PlannedSetupChange struct {
	RootPipeline PlannedFileChange
	Config       PlannedFileChange
	Assets       []PlannedAssetWrite
}
