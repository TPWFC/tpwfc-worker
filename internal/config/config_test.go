package config

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// Helper to create a temp config file.
func createTempConfigFile(t *testing.T, content string) string {
	t.Helper()
	tmpDir := t.TempDir()

	configPath := filepath.Join(tmpDir, "config.yaml")
	if err := os.WriteFile(configPath, []byte(content), 0644); err != nil {
		t.Fatalf("Failed to create temp config file: %v", err)
	}

	return configPath
}

// validConfigYAML is a minimal valid configuration.
const validConfigYAML = `
crawler:
  sources:
    - fire_id: "FIRE001"
      fire_name: "Test Fire"
      language: "en"
      url: "http://example.com/timeline.md"
      enabled: true
  retry:
    max_attempts: 3
    initial_delay_ms: 100
    max_delay_ms: 5000
    backoff_multiplier: 2.0
    timeout_sec: 30
  output:
    base_path: "./output"
    format: "json"
    structure: "fire_language"
    pretty_print: true
  validation:
    validate_table_format: true
    required_fields: ["time", "description"]
    patterns:
      date: "\\d{1,2}月\\d{1,2}日"
      time: "\\d{2}:\\d{2}"
    min_events: 1
    max_events: 1000
  logging:
    level: "info"
    show_progress: true
features:
  enable_caching: true
  strict_validation: false
advanced:
  max_memory_mb: 512
  continue_on_validation_errors: false
`

func TestLoadConfig_Valid(t *testing.T) {
	configPath := createTempConfigFile(t, validConfigYAML)

	cfg, err := LoadConfig(configPath)
	if err != nil {
		t.Fatalf("LoadConfig failed: %v", err)
	}

	if cfg == nil {
		t.Fatal("Expected config, got nil")
	}

	if len(cfg.Crawler.Sources) != 1 {
		t.Errorf("Expected 1 source, got %d", len(cfg.Crawler.Sources))
	}

	if cfg.Crawler.Sources[0].FireID != "FIRE001" {
		t.Errorf("Expected FireID 'FIRE001', got '%s'", cfg.Crawler.Sources[0].FireID)
	}
}

func TestLoadConfig_FileNotFound(t *testing.T) {
	_, err := LoadConfig("/nonexistent/path/config.yaml")
	if err == nil {
		t.Fatal("Expected error for nonexistent file, got nil")
	}
}

func TestLoadConfig_InvalidYAML(t *testing.T) {
	configPath := createTempConfigFile(t, "invalid: yaml: content: [}")

	_, err := LoadConfig(configPath)
	if err == nil {
		t.Fatal("Expected error for invalid YAML, got nil")
	}
}

func TestConfig_Validate_NoSources(t *testing.T) {
	cfg := &Config{
		Crawler: CrawlerConfig{
			Sources: []SourceConfig{},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected validation error for no sources")
	}
}

func TestConfig_Validate_NoEnabledSources(t *testing.T) {
	cfg := &Config{
		Crawler: CrawlerConfig{
			Sources: []SourceConfig{
				{FireID: "FIRE001", Language: "en", URL: "http://example.com", Enabled: false},
			},
			Retry:      RetryPolicy{MaxAttempts: 1, InitialDelayMs: 100, BackoffMultiplier: 1.0, TimeoutSec: 10},
			Output:     OutputConfig{BasePath: "./out", Format: "json"},
			Validation: ValidationConfig{MinEvents: 0, MaxEvents: 100},
			Logging:    LoggingConfig{Level: "info"},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected validation error for no enabled sources")
	}
}

func TestConfig_Validate_MissingURL(t *testing.T) {
	cfg := &Config{
		Crawler: CrawlerConfig{
			Sources: []SourceConfig{
				{FireID: "FIRE001", Language: "en", Enabled: true}, // no URL or File
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected validation error for missing URL/File")
	}
}

func TestConfig_Validate_MissingFireID(t *testing.T) {
	cfg := &Config{
		Crawler: CrawlerConfig{
			Sources: []SourceConfig{
				{Language: "en", URL: "http://example.com", Enabled: true},
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected validation error for missing FireID")
	}
}

func TestConfig_Validate_MissingLanguage(t *testing.T) {
	cfg := &Config{
		Crawler: CrawlerConfig{
			Sources: []SourceConfig{
				{FireID: "FIRE001", URL: "http://example.com", Enabled: true},
			},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected validation error for missing Language")
	}
}

func TestConfig_Validate_InvalidRetryMaxAttempts(t *testing.T) {
	cfg := &Config{
		Crawler: CrawlerConfig{
			Sources: []SourceConfig{
				{FireID: "FIRE001", Language: "en", URL: "http://example.com", Enabled: true},
			},
			Retry: RetryPolicy{MaxAttempts: 0},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected validation error for invalid max_attempts")
	}
}

func TestConfig_Validate_InvalidBackoffMultiplier(t *testing.T) {
	cfg := &Config{
		Crawler: CrawlerConfig{
			Sources: []SourceConfig{
				{FireID: "FIRE001", Language: "en", URL: "http://example.com", Enabled: true},
			},
			Retry: RetryPolicy{MaxAttempts: 1, InitialDelayMs: 100, BackoffMultiplier: 0.5, TimeoutSec: 10},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected validation error for backoff_multiplier < 1.0")
	}
}

func TestConfig_Validate_InvalidOutputFormat(t *testing.T) {
	cfg := &Config{
		Crawler: CrawlerConfig{
			Sources: []SourceConfig{
				{FireID: "FIRE001", Language: "en", URL: "http://example.com", Enabled: true},
			},
			Retry:  RetryPolicy{MaxAttempts: 1, InitialDelayMs: 100, BackoffMultiplier: 1.0, TimeoutSec: 10},
			Output: OutputConfig{BasePath: "./out", Format: "xml"},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected validation error for invalid output format")
	}
}

func TestConfig_Validate_InvalidLoggingLevel(t *testing.T) {
	cfg := &Config{
		Crawler: CrawlerConfig{
			Sources: []SourceConfig{
				{FireID: "FIRE001", Language: "en", URL: "http://example.com", Enabled: true},
			},
			Retry:      RetryPolicy{MaxAttempts: 1, InitialDelayMs: 100, BackoffMultiplier: 1.0, TimeoutSec: 10},
			Output:     OutputConfig{BasePath: "./out", Format: "json"},
			Validation: ValidationConfig{MinEvents: 0, MaxEvents: 100},
			Logging:    LoggingConfig{Level: "verbose"},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected validation error for invalid logging level")
	}
}

func TestConfig_Validate_InvalidRegexPattern(t *testing.T) {
	cfg := &Config{
		Crawler: CrawlerConfig{
			Sources: []SourceConfig{
				{FireID: "FIRE001", Language: "en", URL: "http://example.com", Enabled: true},
			},
			Retry:  RetryPolicy{MaxAttempts: 1, InitialDelayMs: 100, BackoffMultiplier: 1.0, TimeoutSec: 10},
			Output: OutputConfig{BasePath: "./out", Format: "json"},
			Validation: ValidationConfig{
				MinEvents: 0,
				MaxEvents: 100,
				Patterns:  PatternsConfig{Date: "[invalid(regex"},
			},
			Logging: LoggingConfig{Level: "info"},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected validation error for invalid regex pattern")
	}
}

func TestConfig_Validate_MinEventsExceedsMax(t *testing.T) {
	cfg := &Config{
		Crawler: CrawlerConfig{
			Sources: []SourceConfig{
				{FireID: "FIRE001", Language: "en", URL: "http://example.com", Enabled: true},
			},
			Retry:      RetryPolicy{MaxAttempts: 1, InitialDelayMs: 100, BackoffMultiplier: 1.0, TimeoutSec: 10},
			Output:     OutputConfig{BasePath: "./out", Format: "json"},
			Validation: ValidationConfig{MinEvents: 100, MaxEvents: 10},
			Logging:    LoggingConfig{Level: "info"},
		},
	}

	err := cfg.Validate()
	if err == nil {
		t.Fatal("Expected validation error when min_events > max_events")
	}
}

// --- SourceConfig Tests ---

func TestSourceConfig_IsLocalFile(t *testing.T) {
	tests := []struct {
		name     string
		src      SourceConfig
		expected bool
	}{
		{"URL only", SourceConfig{URL: "http://example.com"}, false},
		{"File only", SourceConfig{File: "/path/to/file.md"}, true},
		{"Both URL and File", SourceConfig{URL: "http://example.com", File: "/path/to/file.md"}, true},
		{"Neither", SourceConfig{}, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.src.IsLocalFile(); got != tt.expected {
				t.Errorf("IsLocalFile() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSourceConfig_GetSource(t *testing.T) {
	tests := []struct {
		name     string
		src      SourceConfig
		expected string
	}{
		{"URL only", SourceConfig{URL: "http://example.com"}, "http://example.com"},
		{"File only", SourceConfig{File: "/path/to/file.md"}, "/path/to/file.md"},
		{"Both (File takes precedence)", SourceConfig{URL: "http://example.com", File: "/path/to/file.md"}, "/path/to/file.md"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := tt.src.GetSource(); got != tt.expected {
				t.Errorf("GetSource() = %v, want %v", got, tt.expected)
			}
		})
	}
}

func TestSourceConfig_GetAllURLs(t *testing.T) {
	src := SourceConfig{
		URL:        "http://primary.com",
		BackupURLs: []string{"http://backup1.com", "http://backup2.com"},
	}

	urls := src.GetAllURLs()
	if len(urls) != 3 {
		t.Fatalf("Expected 3 URLs, got %d", len(urls))
	}

	if urls[0] != "http://primary.com" {
		t.Errorf("Expected primary URL first, got %s", urls[0])
	}
}

// --- RetryPolicy Tests ---

func TestRetryPolicy_GetRetryDelay(t *testing.T) {
	rp := RetryPolicy{
		InitialDelayMs:    100,
		MaxDelayMs:        1000,
		BackoffMultiplier: 2.0,
	}

	// The implementation applies multiplier for each retry after the first.
	// Attempt 1: no delay (first attempt)
	// Attempt 2: 100 * 2.0 = 200ms
	// Attempt 3: 200 * 2.0 = 400ms
	// etc.
	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{1, 0},                        // First attempt, no delay
		{2, 200 * time.Millisecond},   // 100 * 2
		{3, 400 * time.Millisecond},   // 100 * 2 * 2
		{4, 800 * time.Millisecond},   // 100 * 2 * 2 * 2
		{5, 1000 * time.Millisecond},  // Capped at max
		{6, 1000 * time.Millisecond},  // Still capped
		{10, 1000 * time.Millisecond}, // Still capped
	}

	for _, tt := range tests {
		t.Run("", func(t *testing.T) {
			got := rp.GetRetryDelay(tt.attempt)
			if got != tt.expected {
				t.Errorf("GetRetryDelay(%d) = %v, want %v", tt.attempt, got, tt.expected)
			}
		})
	}
}

func TestRetryPolicy_GetTimeout(t *testing.T) {
	rp := RetryPolicy{TimeoutSec: 30}
	expected := 30 * time.Second

	if got := rp.GetTimeout(); got != expected {
		t.Errorf("GetTimeout() = %v, want %v", got, expected)
	}
}

// --- Config Helper Method Tests ---

func TestConfig_GetEnabledSources(t *testing.T) {
	cfg := &Config{
		Crawler: CrawlerConfig{
			Sources: []SourceConfig{
				{FireID: "FIRE001", Enabled: true},
				{FireID: "FIRE002", Enabled: false},
				{FireID: "FIRE003", Enabled: true},
			},
		},
	}

	enabled := cfg.GetEnabledSources()
	if len(enabled) != 2 {
		t.Fatalf("Expected 2 enabled sources, got %d", len(enabled))
	}
}

func TestConfig_GetOutputPath(t *testing.T) {
	cfg := &Config{
		Crawler: CrawlerConfig{
			Output: OutputConfig{
				BasePath: "./data",
				Format:   "json",
			},
		},
	}

	path := cfg.GetOutputPath("FIRE001", "zh-HK")
	expected := "./data/FIRE001/zh-HK/timeline.json"

	if path != expected {
		t.Errorf("GetOutputPath() = %v, want %v", path, expected)
	}
}

func TestConfig_GetOutputPath_LegacyFallback(t *testing.T) {
	cfg := &Config{
		Crawler: CrawlerConfig{
			Output: OutputConfig{
				Path:   "./legacy/output.json",
				Format: "json",
			},
		},
	}

	path := cfg.GetOutputPath("FIRE001", "en")
	if path != "./legacy/output.json" {
		t.Errorf("Expected legacy path fallback, got %v", path)
	}
}

func TestConfig_GetSourcesByFire(t *testing.T) {
	cfg := &Config{
		Crawler: CrawlerConfig{
			Sources: []SourceConfig{
				{FireID: "FIRE001", Language: "en", Enabled: true},
				{FireID: "FIRE001", Language: "zh-HK", Enabled: true},
				{FireID: "FIRE002", Language: "en", Enabled: true},
				{FireID: "FIRE001", Language: "zh-CN", Enabled: false},
			},
		},
	}

	sources := cfg.GetSourcesByFire("FIRE001")
	if len(sources) != 2 {
		t.Errorf("Expected 2 sources for FIRE001, got %d", len(sources))
	}
}

func TestConfig_String(t *testing.T) {
	cfg := &Config{
		Crawler: CrawlerConfig{
			Sources: []SourceConfig{{}, {}, {}},
			Retry:   RetryPolicy{MaxAttempts: 5},
			Output:  OutputConfig{BasePath: "./output"},
		},
	}

	str := cfg.String()
	if str == "" {
		t.Error("Expected non-empty string representation")
	}
}

func TestConfig_SaveConfig(t *testing.T) {
	cfg := &Config{
		Crawler: CrawlerConfig{
			Sources: []SourceConfig{
				{FireID: "FIRE001", Language: "en", URL: "http://example.com", Enabled: true},
			},
			Retry:      RetryPolicy{MaxAttempts: 3, InitialDelayMs: 100, BackoffMultiplier: 1.5, TimeoutSec: 30},
			Output:     OutputConfig{BasePath: "./out", Format: "json"},
			Validation: ValidationConfig{MinEvents: 0, MaxEvents: 100},
			Logging:    LoggingConfig{Level: "info"},
		},
	}

	tmpDir := t.TempDir()
	savePath := filepath.Join(tmpDir, "saved_config.yaml")

	err := cfg.SaveConfig(savePath)
	if err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	// Verify file was created
	if _, statErr := os.Stat(savePath); os.IsNotExist(statErr) {
		t.Fatal("Expected saved config file to exist")
	}

	// Verify we can load it back
	loaded, err := LoadConfig(savePath)
	if err != nil {
		t.Fatalf("Failed to load saved config: %v", err)
	}

	if loaded.Crawler.Sources[0].FireID != "FIRE001" {
		t.Error("Loaded config does not match saved config")
	}
}
