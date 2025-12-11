package validator

import (
	"strings"
	"testing"

	"tpwfc/internal/config"
)

// Helper to create a valid config for testing.
func createTestConfig(t *testing.T) *config.Config {
	t.Helper()

	return &config.Config{
		Crawler: config.CrawlerConfig{
			Validation: config.ValidationConfig{
				ValidateTableFormat: true,
				RequiredFields:      []string{"time", "description"},
				Patterns: config.PatternsConfig{
					Date: `\d{4}-\d{2}-\d{2}`,
					Time: `\d{2}:\d{2}`,
				},
				MinEvents: 1,
				MaxEvents: 1000,
			},
			Logging: config.LoggingConfig{
				Level:              "info",
				DetailedValidation: false,
			},
		},
	}
}

func TestNewMarkdownValidator(t *testing.T) {
	cfg := createTestConfig(t)

	v, err := NewMarkdownValidator(cfg)
	if err != nil {
		t.Fatalf("NewMarkdownValidator failed: %v", err)
	}

	if v == nil {
		t.Fatal("NewMarkdownValidator returned nil")
	}
}

func TestNewMarkdownValidator_InvalidDatePattern(t *testing.T) {
	cfg := &config.Config{
		Crawler: config.CrawlerConfig{
			Validation: config.ValidationConfig{
				Patterns: config.PatternsConfig{
					Date: "[invalid(regex",
				},
			},
		},
	}

	_, err := NewMarkdownValidator(cfg)
	if err == nil {
		t.Fatal("Expected error for invalid date pattern")
	}
}

func TestNewMarkdownValidator_InvalidTimePattern(t *testing.T) {
	cfg := &config.Config{
		Crawler: config.CrawlerConfig{
			Validation: config.ValidationConfig{
				Patterns: config.PatternsConfig{
					Time: "[invalid(regex",
				},
			},
		},
	}

	_, err := NewMarkdownValidator(cfg)
	if err == nil {
		t.Fatal("Expected error for invalid time pattern")
	}
}

func TestNewMarkdownValidator_InvalidDescriptionPattern(t *testing.T) {
	cfg := &config.Config{
		Crawler: config.CrawlerConfig{
			Validation: config.ValidationConfig{
				Patterns: config.PatternsConfig{
					Description: "[invalid(regex",
				},
			},
		},
	}

	_, err := NewMarkdownValidator(cfg)
	if err == nil {
		t.Fatal("Expected error for invalid description pattern")
	}
}

func TestNewMarkdownValidator_InvalidCasualtiesPattern(t *testing.T) {
	cfg := &config.Config{
		Crawler: config.CrawlerConfig{
			Validation: config.ValidationConfig{
				MinCasualtiesPattern: "[invalid(regex",
			},
		},
	}

	_, err := NewMarkdownValidator(cfg)
	if err == nil {
		t.Fatal("Expected error for invalid casualties pattern")
	}
}

func TestValidateMarkdown_ValidationDisabled(t *testing.T) {
	cfg := &config.Config{
		Crawler: config.CrawlerConfig{
			Validation: config.ValidationConfig{
				ValidateTableFormat: false,
			},
		},
	}

	v, err := NewMarkdownValidator(cfg)
	if err != nil {
		t.Fatalf("NewMarkdownValidator failed: %v", err)
	}

	result := v.ValidateMarkdown("any content")
	if !result.IsValid {
		t.Error("Expected valid result when validation is disabled")
	}
}

func TestValidateMarkdown_ValidTable(t *testing.T) {
	cfg := createTestConfig(t)

	v, err := NewMarkdownValidator(cfg)
	if err != nil {
		t.Fatalf("NewMarkdownValidator failed: %v", err)
	}

	markdown := `
| DATE | TIME | EVENT |
|------|------|-------|
| 2024-11-26 | 10:30 | First event description |
| 2024-11-26 | 11:45 | Second event description |
`

	result := v.ValidateMarkdown(markdown)
	if !result.IsValid {
		t.Errorf("Expected valid markdown, got errors: %v", result.Errors)
	}

	if result.Stats.TotalRows != 2 {
		t.Errorf("Expected 2 total rows, got %d", result.Stats.TotalRows)
	}

	if result.Stats.ValidRows != 2 {
		t.Errorf("Expected 2 valid rows, got %d", result.Stats.ValidRows)
	}
}

func TestValidateMarkdown_EmptyTimeField(t *testing.T) {
	cfg := createTestConfig(t)

	v, err := NewMarkdownValidator(cfg)
	if err != nil {
		t.Fatalf("NewMarkdownValidator failed: %v", err)
	}

	markdown := `
| DATE | TIME | EVENT |
|------|------|-------|
| 2024-11-26 |  | Event with empty time |
`

	result := v.ValidateMarkdown(markdown)
	if result.IsValid {
		t.Error("Expected invalid result for empty time field")
	}

	if result.Stats.InvalidRows != 1 {
		t.Errorf("Expected 1 invalid row, got %d", result.Stats.InvalidRows)
	}
}

func TestValidateMarkdown_InvalidTimeFormat(t *testing.T) {
	cfg := createTestConfig(t)

	v, err := NewMarkdownValidator(cfg)
	if err != nil {
		t.Fatalf("NewMarkdownValidator failed: %v", err)
	}

	markdown := `
| DATE | TIME | EVENT |
|------|------|-------|
| 2024-11-26 | 9am | Event with invalid time format |
`

	result := v.ValidateMarkdown(markdown)
	if result.IsValid {
		t.Error("Expected invalid result for wrong time format")
	}

	// Check that error mentions time
	foundTimeError := false

	for _, err := range result.Errors {
		if err.Field == "time" {
			foundTimeError = true

			break
		}
	}

	if !foundTimeError {
		t.Error("Expected time validation error")
	}
}

func TestValidateMarkdown_EmptyDescription(t *testing.T) {
	cfg := createTestConfig(t)

	v, err := NewMarkdownValidator(cfg)
	if err != nil {
		t.Fatalf("NewMarkdownValidator failed: %v", err)
	}

	markdown := `
| DATE | TIME | EVENT |
|------|------|-------|
| 2024-11-26 | 10:30 |  |
`

	result := v.ValidateMarkdown(markdown)
	if result.IsValid {
		t.Error("Expected invalid result for empty description")
	}
}

func TestValidateMarkdown_TooFewColumns(t *testing.T) {
	cfg := createTestConfig(t)

	v, err := NewMarkdownValidator(cfg)
	if err != nil {
		t.Fatalf("NewMarkdownValidator failed: %v", err)
	}

	markdown := `
| DATE | TIME |
|------|------|
| 2024-11-26 | 10:30 |
`

	result := v.ValidateMarkdown(markdown)
	if result.IsValid {
		t.Error("Expected invalid result for too few columns")
	}
}

func TestValidateMarkdown_MinEventsNotMet(t *testing.T) {
	cfg := createTestConfig(t)
	cfg.Crawler.Validation.MinEvents = 5

	v, err := NewMarkdownValidator(cfg)
	if err != nil {
		t.Fatalf("NewMarkdownValidator failed: %v", err)
	}

	markdown := `
| DATE | TIME | EVENT |
|------|------|-------|
| 2024-11-26 | 10:30 | Single event |
`

	result := v.ValidateMarkdown(markdown)

	// Should have error about minimum events
	foundMinError := false

	for _, err := range result.Errors {
		if strings.Contains(err.Message, "minimum events") {
			foundMinError = true

			break
		}
	}

	if !foundMinError {
		t.Error("Expected minimum events error")
	}
}

func TestValidateMarkdown_MaxEventsExceeded(t *testing.T) {
	cfg := createTestConfig(t)
	cfg.Crawler.Validation.MaxEvents = 1

	v, err := NewMarkdownValidator(cfg)
	if err != nil {
		t.Fatalf("NewMarkdownValidator failed: %v", err)
	}

	markdown := `
| DATE | TIME | EVENT |
|------|------|-------|
| 2024-11-26 | 10:30 | First event |
| 2024-11-26 | 11:00 | Second event |
| 2024-11-26 | 12:00 | Third event |
`

	result := v.ValidateMarkdown(markdown)

	// Should have warning about max events
	foundMaxWarning := false

	for _, warn := range result.Warnings {
		if strings.Contains(warn, "high event count") {
			foundMaxWarning = true

			break
		}
	}

	if !foundMaxWarning {
		t.Error("Expected max events warning")
	}
}

func TestValidateSingleRow_Valid(t *testing.T) {
	cfg := createTestConfig(t)

	v, err := NewMarkdownValidator(cfg)
	if err != nil {
		t.Fatalf("NewMarkdownValidator failed: %v", err)
	}

	err = v.ValidateSingleRow("10:30", "2024-11-26", "Event description")
	if err != nil {
		t.Errorf("ValidateSingleRow returned unexpected error: %v", err)
	}
}

func TestValidateSingleRow_EmptyTime(t *testing.T) {
	cfg := createTestConfig(t)

	v, err := NewMarkdownValidator(cfg)
	if err != nil {
		t.Fatalf("NewMarkdownValidator failed: %v", err)
	}

	err = v.ValidateSingleRow("", "2024-11-26", "Event description")
	if err == nil {
		t.Error("Expected error for empty time")
	}
}

func TestValidateSingleRow_EmptyDescription(t *testing.T) {
	cfg := createTestConfig(t)

	v, err := NewMarkdownValidator(cfg)
	if err != nil {
		t.Fatalf("NewMarkdownValidator failed: %v", err)
	}

	err = v.ValidateSingleRow("10:30", "2024-11-26", "")
	if err == nil {
		t.Error("Expected error for empty description")
	}
}

func TestValidateSingleRow_InvalidTimeFormat(t *testing.T) {
	cfg := createTestConfig(t)

	v, err := NewMarkdownValidator(cfg)
	if err != nil {
		t.Fatalf("NewMarkdownValidator failed: %v", err)
	}

	err = v.ValidateSingleRow("9am", "2024-11-26", "Event description")
	if err == nil {
		t.Error("Expected error for invalid time format")
	}
}

func TestValidateSingleRow_InvalidDateFormat(t *testing.T) {
	cfg := createTestConfig(t)

	v, err := NewMarkdownValidator(cfg)
	if err != nil {
		t.Fatalf("NewMarkdownValidator failed: %v", err)
	}

	err = v.ValidateSingleRow("10:30", "11月26日", "Event description")
	if err == nil {
		t.Error("Expected error for invalid date format")
	}
}

// --- Helper function tests ---

func TestExtractDateFromHeader(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"**11月26日**", "11月26日"},
		{"**12月1日**", "12月1日"},
		{"11月26日", "11月26日"}, // Without ** markers
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := extractDateFromHeader(tt.input)
			if got != tt.expected {
				t.Errorf("extractDateFromHeader(%q) = %q, want %q", tt.input, got, tt.expected)
			}
		})
	}
}

func TestTruncate(t *testing.T) {
	tests := []struct {
		input    string
		expected string
		maxLen   int
	}{
		{"short", "short", 10},
		{"exactly10!", "exactly10!", 10},
		{"this is a long string", "this is a ...", 10},
		{"", "", 5},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := truncate(tt.input, tt.maxLen)
			if got != tt.expected {
				t.Errorf("truncate(%q, %d) = %q, want %q", tt.input, tt.maxLen, got, tt.expected)
			}
		})
	}
}

// --- ValidationResult tests ---

func TestValidationResult_String(t *testing.T) {
	result := &ValidationResult{
		IsValid: true,
		Stats: ValidationStats{
			TotalRows:   10,
			ValidRows:   10,
			InvalidRows: 0,
		},
		Warnings: []string{},
	}

	str := result.String()
	if !strings.Contains(str, "VALID") {
		t.Error("Expected 'VALID' in string representation")
	}

	if !strings.Contains(str, "10") {
		t.Error("Expected row count in string representation")
	}
}

func TestValidationResult_String_Invalid(t *testing.T) {
	result := &ValidationResult{
		IsValid: false,
		Stats: ValidationStats{
			TotalRows:   10,
			ValidRows:   5,
			InvalidRows: 5,
		},
	}

	str := result.String()
	if !strings.Contains(str, "INVALID") {
		t.Error("Expected 'INVALID' in string representation")
	}
}
