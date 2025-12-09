package config

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

const (
	defaultMediaBase = "/mnt/media"
	pipelineDirName  = "pipeline"
	configFileName   = "config.yaml"
)

// RemuxConfig holds remux-specific configuration
type RemuxConfig struct {
	Languages []string `yaml:"languages"`
}

// TranscodeConfig holds transcode-specific configuration
type TranscodeConfig struct {
	CRF      int    `yaml:"crf"`       // Quality (0-51, default 20)
	Mode     string `yaml:"mode"`      // "software" or "hardware"
	Preset   string `yaml:"preset"`    // libx265 preset (default "slow")
	HWPreset string `yaml:"hw_preset"` // QSV preset (default "medium")
}

// Config holds application configuration
type Config struct {
	StagingBase string            `yaml:"staging_base"` // Staging directory
	LibraryBase string            `yaml:"library_base"` // Library directory
	Dispatch    map[string]string `yaml:"dispatch"`     // SSH targets per stage
	Remux       RemuxConfig       `yaml:"remux"`        // Remux configuration
	Transcode   TranscodeConfig   `yaml:"transcode"`    // Transcode configuration

	// Derived from environment, not stored in YAML
	mediaBase string
}

// MediaBase returns the MEDIA_BASE path
func (c *Config) MediaBase() string {
	if c.mediaBase != "" {
		return c.mediaBase
	}
	if base := os.Getenv("MEDIA_BASE"); base != "" {
		return base
	}
	return defaultMediaBase
}

// DataDir returns the pipeline data directory ($MEDIA_BASE/pipeline)
func (c *Config) DataDir() string {
	return filepath.Join(c.MediaBase(), pipelineDirName)
}

// DatabasePath returns the path to the SQLite database
func (c *Config) DatabasePath() string {
	return filepath.Join(c.DataDir(), "pipeline.db")
}

// JobLogDir returns the directory for a job's log files
func (c *Config) JobLogDir(jobID int64) string {
	return filepath.Join(c.DataDir(), "logs", "jobs", fmt.Sprintf("%d", jobID))
}

// JobLogPath returns the path for a job's main log file
func (c *Config) JobLogPath(jobID int64) string {
	return filepath.Join(c.JobLogDir(jobID), "job.log")
}

// ToolLogPath returns the path for a tool's raw log file
func (c *Config) ToolLogPath(jobID int64, tool string) string {
	return filepath.Join(c.JobLogDir(jobID), fmt.Sprintf("%s.log", tool))
}

// EnsureJobLogDir creates the log directory for a specific job
func (c *Config) EnsureJobLogDir(jobID int64) error {
	return os.MkdirAll(c.JobLogDir(jobID), 0755)
}

// DispatchTarget returns the SSH target for a stage, or empty for local execution
func (c *Config) DispatchTarget(stage string) string {
	if c.Dispatch == nil {
		return ""
	}
	return c.Dispatch[stage]
}

// IsLocal returns true if the stage should run locally (no SSH)
func (c *Config) IsLocal(stage string) bool {
	return c.DispatchTarget(stage) == ""
}

// RemuxLanguages returns the list of languages to keep during remux
// Defaults to ["eng"] if not configured
func (c *Config) RemuxLanguages() []string {
	if len(c.Remux.Languages) == 0 {
		return []string{"eng"}
	}
	return c.Remux.Languages
}

// TranscodeCRF returns the CRF value for transcoding
// Defaults to 20 if not configured
func (c *Config) TranscodeCRF() int {
	if c.Transcode.CRF == 0 {
		return 20
	}
	return c.Transcode.CRF
}

// TranscodeMode returns the encoding mode ("software" or "hardware")
// Defaults to "software" if not configured
func (c *Config) TranscodeMode() string {
	if c.Transcode.Mode == "" {
		return "software"
	}
	return c.Transcode.Mode
}

// TranscodePreset returns the libx265 preset
// Defaults to "slow" if not configured
func (c *Config) TranscodePreset() string {
	if c.Transcode.Preset == "" {
		return "slow"
	}
	return c.Transcode.Preset
}

// TranscodeHWPreset returns the QSV preset
// Defaults to "medium" if not configured
func (c *Config) TranscodeHWPreset() string {
	if c.Transcode.HWPreset == "" {
		return "medium"
	}
	return c.Transcode.HWPreset
}

// Load reads configuration from a YAML file
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("failed to read config: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config: %w", err)
	}

	return &cfg, nil
}

// LoadDefault loads config from default location
func LoadDefault() (*Config, error) {
	// Check XDG config dir first
	if xdg := os.Getenv("XDG_CONFIG_HOME"); xdg != "" {
		path := filepath.Join(xdg, "media-pipeline", "config.yaml")
		if _, err := os.Stat(path); err == nil {
			return Load(path)
		}
	}

	// Fall back to ~/.config
	home, err := os.UserHomeDir()
	if err != nil {
		return nil, fmt.Errorf("failed to get home dir: %w", err)
	}

	path := filepath.Join(home, ".config", "media-pipeline", "config.yaml")
	return Load(path)
}

// LoadFromMediaBase loads config from $MEDIA_BASE/pipeline/config.yaml
func LoadFromMediaBase() (*Config, error) {
	mediaBase := os.Getenv("MEDIA_BASE")
	if mediaBase == "" {
		mediaBase = defaultMediaBase
	}

	configPath := filepath.Join(mediaBase, pipelineDirName, configFileName)
	cfg, err := Load(configPath)
	if err != nil {
		return nil, err
	}

	cfg.mediaBase = mediaBase
	return cfg, nil
}
