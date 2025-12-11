// Package config provides configuration management for the crawler worker.
package config

import (
	"errors"
	"fmt"
	"os"
	"regexp"
	"time"

	"gopkg.in/yaml.v3"
)

// Configuration validation errors.
var (
	ErrNoSources                = errors.New("at least one source is required")
	ErrSourceMissingURLOrFile   = errors.New("either URL or file path is required")
	ErrSourceMissingFireID      = errors.New("fire_id is required")
	ErrSourceMissingLanguage    = errors.New("language is required")
	ErrNoEnabledSources         = errors.New("at least one source must be enabled")
	ErrInvalidMaxAttempts       = errors.New("retry.max_attempts must be at least 1")
	ErrInvalidInitialDelay      = errors.New("retry.initial_delay_ms must be non-negative")
	ErrInvalidBackoffMultiplier = errors.New("retry.backoff_multiplier must be >= 1.0")
	ErrInvalidTimeout           = errors.New("retry.timeout_sec must be at least 1")
	ErrMissingOutputPath        = errors.New("output.base_path or output.path is required")
	ErrInvalidOutputFormat      = errors.New("output.format must be 'json' or 'jsonl'")
	ErrInvalidMinEvents         = errors.New("validation.min_events must be non-negative")
	ErrInvalidMaxEvents         = errors.New("validation.max_events must be at least 1")
	ErrMinExceedsMax            = errors.New("validation.min_events cannot exceed validation.max_events")
	ErrInvalidLogLevel          = errors.New("logging.level must be one of: debug, info, warn, error")
)

// Config represents the complete crawler configuration.
type Config struct {
	Crawler  CrawlerConfig  `yaml:"crawler"`
	Features FeaturesConfig `yaml:"features"`
	Advanced AdvancedConfig `yaml:"advanced"`
}

// CrawlerConfig contains crawler-specific settings.
type CrawlerConfig struct {
	Output     OutputConfig     `yaml:"output"`
	Sources    []SourceConfig   `yaml:"sources"`
	Logging    LoggingConfig    `yaml:"logging"`
	Validation ValidationConfig `yaml:"validation"`
	Retry      RetryPolicy      `yaml:"retry"`
}

// SourceConfig represents a timeline source.
type SourceConfig struct {
	FireID     string   `yaml:"fire_id"`
	FireName   string   `yaml:"fire_name"`
	Language   string   `yaml:"language"`
	URL        string   `yaml:"url"`
	File       string   `yaml:"file"`
	Name       string   `yaml:"name"`
	BackupURLs []string `yaml:"backup_urls"`
	Enabled    bool     `yaml:"enabled"`
}

// IsLocalFile returns true if this source uses a local file.
func (s *SourceConfig) IsLocalFile() bool {
	return s.File != ""
}

// GetSource returns the file path if local, or URL if remote.
func (s *SourceConfig) GetSource() string {
	if s.IsLocalFile() {
		return s.File
	}

	return s.URL
}

// RetryPolicy defines retry behavior.
type RetryPolicy struct {
	MaxAttempts       int     `yaml:"max_attempts"`
	InitialDelayMs    int     `yaml:"initial_delay_ms"`
	MaxDelayMs        int     `yaml:"max_delay_ms"`
	BackoffMultiplier float64 `yaml:"backoff_multiplier"`
	TimeoutSec        int     `yaml:"timeout_sec"`
}

// OutputConfig defines output behavior.
type OutputConfig struct {
	BasePath     string `yaml:"base_path"`
	Format       string `yaml:"format"`
	Structure    string `yaml:"structure"`
	Path         string `yaml:"path"`
	PrettyPrint  bool   `yaml:"pretty_print"`
	CreateBackup bool   `yaml:"create_backup"`
}

// ValidationConfig defines markdown validation rules.
type ValidationConfig struct {
	Patterns             PatternsConfig `yaml:"patterns"`
	MinCasualtiesPattern string         `yaml:"min_casualties_pattern"`
	RequiredFields       []string       `yaml:"required_fields"`
	MinEvents            int            `yaml:"min_events"`
	MaxEvents            int            `yaml:"max_events"`
	ValidateTableFormat  bool           `yaml:"validate_table_format"`
	ValidateCasualties   bool           `yaml:"validate_casualties"`
}

// PatternsConfig defines regex patterns for validation.
type PatternsConfig struct {
	Date        string `yaml:"date"`
	Time        string `yaml:"time"`
	Description string `yaml:"description"`
}

// LoggingConfig defines logging behavior.
type LoggingConfig struct {
	Level              string `yaml:"level"`
	SampleEvents       int    `yaml:"sample_events"`
	ShowProgress       bool   `yaml:"show_progress"`
	DetailedValidation bool   `yaml:"detailed_validation"`
}

// FeaturesConfig contains feature flags.
type FeaturesConfig struct {
	EnableCaching              bool `yaml:"enable_caching"`
	EnableNormalizationPreview bool `yaml:"enable_normalization_preview"`
	StrictValidation           bool `yaml:"strict_validation"`
	EnableMarkdownFormatter    bool `yaml:"enable_markdown_formatter"`
}

// AdvancedConfig contains advanced settings.
type AdvancedConfig struct {
	MaxMemoryMb                int  `yaml:"max_memory_mb"`
	ConcurrentURLAttempts      bool `yaml:"concurrent_url_attempts"`
	ContinueOnValidationErrors bool `yaml:"continue_on_validation_errors"`
	SaveFailedRows             bool `yaml:"save_failed_rows"`
	BufferSizeKb               int  `yaml:"buffer_size_kb"`
}

// LoadConfig loads configuration from YAML file.
func LoadConfig(filepath string) (*Config, error) {
	data, err := os.ReadFile(filepath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("configuration validation failed: %w", err)
	}

	return &cfg, nil
}

// SaveConfig saves configuration to YAML file.
func (c *Config) SaveConfig(filepath string) error {
	data, err := yaml.Marshal(c)
	if err != nil {
		return fmt.Errorf("failed to marshal config: %w", err)
	}

	if err := os.WriteFile(filepath, data, 0644); err != nil {
		return fmt.Errorf("failed to write config file: %w", err)
	}

	return nil
}

// Validate validates the configuration.
func (c *Config) Validate() error {
	// Check crawler config
	if len(c.Crawler.Sources) == 0 {
		return ErrNoSources
	}

	enabledCount := 0

	for i, src := range c.Crawler.Sources {
		// Either URL or File must be provided
		if src.URL == "" && src.File == "" {
			return fmt.Errorf("%w: source[%d]", ErrSourceMissingURLOrFile, i)
		}

		if src.FireID == "" {
			return fmt.Errorf("%w: source[%d]", ErrSourceMissingFireID, i)
		}

		if src.Language == "" {
			return fmt.Errorf("%w: source[%d]", ErrSourceMissingLanguage, i)
		}

		if src.Enabled {
			enabledCount++
		}
	}

	if enabledCount == 0 {
		return ErrNoEnabledSources
	}

	// Validate retry policy
	if c.Crawler.Retry.MaxAttempts < 1 {
		return ErrInvalidMaxAttempts
	}

	if c.Crawler.Retry.InitialDelayMs < 0 {
		return ErrInvalidInitialDelay
	}

	if c.Crawler.Retry.BackoffMultiplier < 1.0 {
		return ErrInvalidBackoffMultiplier
	}

	if c.Crawler.Retry.TimeoutSec < 1 {
		return ErrInvalidTimeout
	}

	// Validate output config
	if c.Crawler.Output.BasePath == "" && c.Crawler.Output.Path == "" {
		return ErrMissingOutputPath
	}

	if c.Crawler.Output.Format != "json" && c.Crawler.Output.Format != "jsonl" {
		return ErrInvalidOutputFormat
	}

	// Validate validation config
	if c.Crawler.Validation.MinEvents < 0 {
		return ErrInvalidMinEvents
	}

	if c.Crawler.Validation.MaxEvents < 1 {
		return ErrInvalidMaxEvents
	}

	if c.Crawler.Validation.MinEvents > c.Crawler.Validation.MaxEvents {
		return ErrMinExceedsMax
	}

	// Validate regex patterns
	patterns := map[string]string{
		"date":        c.Crawler.Validation.Patterns.Date,
		"time":        c.Crawler.Validation.Patterns.Time,
		"description": c.Crawler.Validation.Patterns.Description,
	}

	for name, pattern := range patterns {
		if pattern != "" {
			if _, err := regexp.Compile(pattern); err != nil {
				return fmt.Errorf("validation.patterns.%s is invalid regex: %w", name, err)
			}
		}
	}

	// Validate casualties pattern
	if c.Crawler.Validation.MinCasualtiesPattern != "" {
		if _, err := regexp.Compile(c.Crawler.Validation.MinCasualtiesPattern); err != nil {
			return fmt.Errorf("validation.min_casualties_pattern is invalid regex: %w", err)
		}
	}

	// Validate logging config
	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[c.Crawler.Logging.Level] {
		return ErrInvalidLogLevel
	}

	return nil
}

// GetEnabledSources returns only enabled sources.
func (c *Config) GetEnabledSources() []SourceConfig {
	var enabled []SourceConfig

	for _, src := range c.Crawler.Sources {
		if src.Enabled {
			enabled = append(enabled, src)
		}
	}

	return enabled
}

// GetRetryDelay calculates exponential backoff delay for attempt number.
func (rp *RetryPolicy) GetRetryDelay(attempt int) time.Duration {
	if attempt <= 1 {
		return 0
	}

	delayMs := float64(rp.InitialDelayMs)
	for i := 1; i < attempt; i++ {
		delayMs *= rp.BackoffMultiplier
	}

	// Cap at max delay
	if int(delayMs) > rp.MaxDelayMs {
		delayMs = float64(rp.MaxDelayMs)
	}

	return time.Duration(int(delayMs)) * time.Millisecond
}

// GetTimeout returns the timeout duration.
func (rp *RetryPolicy) GetTimeout() time.Duration {
	return time.Duration(rp.TimeoutSec) * time.Second
}

// GetOutputPath follows structure: {base_path}/{fire_id}/{language}/timeline.{format}.
func (c *Config) GetOutputPath(fireID, language string) string {
	if c.Crawler.Output.BasePath != "" {
		return fmt.Sprintf("%s/%s/%s/timeline.%s",
			c.Crawler.Output.BasePath,
			fireID,
			language,
			c.Crawler.Output.Format,
		)
	}
	// Fallback to legacy path if specified
	return c.Crawler.Output.Path
}

// GetSourcesByFire returns sources for a specific fire incident.
func (c *Config) GetSourcesByFire(fireID string) []SourceConfig {
	var sources []SourceConfig

	for _, src := range c.Crawler.Sources {
		if src.FireID == fireID && src.Enabled {
			sources = append(sources, src)
		}
	}

	return sources
}

// GetAllURLs returns all URLs (primary + backups) for a source.
func (s *SourceConfig) GetAllURLs() []string {
	urls := []string{s.URL}
	urls = append(urls, s.BackupURLs...)

	return urls
}

// String returns a string representation of the config.
func (c *Config) String() string {
	return fmt.Sprintf(
		"Config{Sources: %d, MaxAttempts: %d, Output: %s}",
		len(c.Crawler.Sources),
		c.Crawler.Retry.MaxAttempts,
		c.Crawler.Output.BasePath,
	)
}
