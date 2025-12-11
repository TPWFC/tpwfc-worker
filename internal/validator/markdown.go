// Package validator provides validation utilities for markdown documents.
package validator

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"tpwfc/internal/config"
	"tpwfc/pkg/metadata"
)

// Validation errors.
var (
	ErrTimeRequired        = errors.New("time is required")
	ErrDescriptionRequired = errors.New("description is required")
	ErrInvalidTimeFormat   = errors.New("invalid time format")
	ErrInvalidDateFormat   = errors.New("invalid date format")
	ErrDescriptionPattern  = errors.New("description does not match pattern")
)

// ValidationError represents a validation error with context.
type ValidationError struct {
	Field   string
	Value   string
	Pattern string
	Message string
	Line    int
	Column  int
}

// ValidationResult contains validation results.
type ValidationResult struct {
	Errors   []ValidationError
	Warnings []string
	Stats    ValidationStats
	IsValid  bool
}

// ValidationStats contains validation statistics.
type ValidationStats struct {
	TotalRows           int
	ValidRows           int
	InvalidRows         int
	RowsWithMissing     int
	RowsWithInvalidTime int
	RowsWithInvalidDate int
}

// MarkdownValidator validates markdown format.
type MarkdownValidator struct {
	cfg *config.Config
	// Compiled regex patterns
	datePattern        *regexp.Regexp
	timePattern        *regexp.Regexp
	descriptionPattern *regexp.Regexp
	casualtiesPattern  *regexp.Regexp
}

// NewMarkdownValidator creates a new validator.
func NewMarkdownValidator(cfg *config.Config) (*MarkdownValidator, error) {
	v := &MarkdownValidator{cfg: cfg}

	// Compile regex patterns
	var err error
	if cfg.Crawler.Validation.Patterns.Date != "" {
		v.datePattern, err = regexp.Compile(cfg.Crawler.Validation.Patterns.Date)
		if err != nil {
			return nil, fmt.Errorf("invalid date pattern: %w", err)
		}
	}

	if cfg.Crawler.Validation.Patterns.Time != "" {
		v.timePattern, err = regexp.Compile(cfg.Crawler.Validation.Patterns.Time)
		if err != nil {
			return nil, fmt.Errorf("invalid time pattern: %w", err)
		}
	}

	if cfg.Crawler.Validation.Patterns.Description != "" {
		v.descriptionPattern, err = regexp.Compile(cfg.Crawler.Validation.Patterns.Description)
		if err != nil {
			return nil, fmt.Errorf("invalid description pattern: %w", err)
		}
	}

	if cfg.Crawler.Validation.MinCasualtiesPattern != "" {
		v.casualtiesPattern, err = regexp.Compile(cfg.Crawler.Validation.MinCasualtiesPattern)
		if err != nil {
			return nil, fmt.Errorf("invalid casualties pattern: %w", err)
		}
	}

	return v, nil
}

// ValidateMarkdown validates markdown table format.
func (v *MarkdownValidator) ValidateMarkdown(markdown string) *ValidationResult {
	result := &ValidationResult{
		IsValid:  true,
		Errors:   []ValidationError{},
		Warnings: []string{},
		Stats:    ValidationStats{},
	}

	if !v.cfg.Crawler.Validation.ValidateTableFormat {
		return result
	}

	lines := strings.Split(markdown, "\n")

	// Find table rows (skip headers and separators)
	tableStarted := false
	rowNumber := 0

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			tableStarted = false
			continue
		}

		// Skip markdown table separators
		if strings.HasPrefix(line, "|") && strings.Contains(line, "---") {
			continue
		}

		// Check for Table Headers explicitly
		if strings.HasPrefix(line, "|") {
			// Check if this is the Timeline Table header
			// Only check if we are NOT currently in a table, to prevent false positives from row content
			if !tableStarted {
				upperLine := strings.ToUpper(line)
				if strings.Contains(upperLine, "時間") || strings.Contains(upperLine, "TIME") || strings.Contains(upperLine, "DATE") || strings.Contains(upperLine, "日期") || strings.Contains(upperLine, "时间") {
					// Only enable validation for the TIMELINE table
					// We assume other tables (KEY/VALUE) don't have these specific headers in this combination
					if strings.Contains(upperLine, "EVENT") || strings.Contains(upperLine, "事件") {
						tableStarted = true
						continue
					}
				}
			}

			// If we are in a table but it's not the timeline table (e.g. KEY|VALUE), tableStarted should be false from the loop reset or not set yet
			if !tableStarted {
				continue
			}

			// Process table data rows
			rowNumber++
			result.Stats.TotalRows++

			rowError := v.validateRow(line, lineNum+1, rowNumber)
			if len(rowError) > 0 {
				result.IsValid = false
				result.Stats.InvalidRows++
				result.Errors = append(result.Errors, rowError...)
			} else {
				result.Stats.ValidRows++
			}
		} else {
			// Not a table line
			tableStarted = false
		}
	}

	// Check minimum/maximum events
	if result.Stats.ValidRows < v.cfg.Crawler.Validation.MinEvents {
		result.IsValid = v.cfg.Crawler.Validation.MinEvents == 0
		result.Errors = append(result.Errors, ValidationError{
			Message: fmt.Sprintf(
				"minimum events not met: got %d, expected at least %d",
				result.Stats.ValidRows,
				v.cfg.Crawler.Validation.MinEvents,
			),
		})
	}

	if result.Stats.ValidRows > v.cfg.Crawler.Validation.MaxEvents {
		result.Warnings = append(result.Warnings,
			fmt.Sprintf(
				"unusually high event count: got %d, expected max %d (check for parsing errors)",
				result.Stats.ValidRows,
				v.cfg.Crawler.Validation.MaxEvents,
			),
		)
	}

	return result
}

// ValidateIntegrity checks the integrity of the markdown content using the metadata block.
func (v *MarkdownValidator) ValidateIntegrity(content string) *ValidationResult {
	result := &ValidationResult{
		IsValid: true,
		Stats:   ValidationStats{},
	}

	valid, err := metadata.Verify(content)
	if !valid {
		result.IsValid = false
		result.Errors = append(result.Errors, ValidationError{
			Message: fmt.Sprintf("integrity check failed: %v", err),
		})
	}

	return result
}

// validateRow validates a single table row.
func (v *MarkdownValidator) validateRow(row string, lineNum int, rowNum int) []ValidationError {
	var errs []ValidationError

	// Split row by pipes
	cells := strings.Split(row, "|")

	// Remove leading/trailing empty cells
	var values []string
	for i := 1; i < len(cells)-1; i++ {
		values = append(values, strings.TrimSpace(cells[i]))
	}

	// Expect: DATE | TIME | DESCRIPTION | ...
	if len(values) < 3 {
		errs = append(errs, ValidationError{
			Line:    lineNum,
			Column:  1,
			Message: fmt.Sprintf("expected at least 3 columns (Date, Time, Event), got %d", len(values)),
		})

		return errs
	}

	// 1. Validate DATE (YYYY-MM-DD)
	dateVal := values[0]
	dateRegex := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

	if dateVal == "" {
		errs = append(errs, ValidationError{
			Line:    lineNum,
			Column:  1,
			Field:   "date",
			Message: "date field is empty",
		})
	} else if !dateRegex.MatchString(dateVal) {
		errs = append(errs, ValidationError{
			Line:    lineNum,
			Column:  1,
			Field:   "date",
			Value:   dateVal,
			Pattern: "YYYY-MM-DD",
			Message: fmt.Sprintf("date '%s' invalid format", dateVal),
		})
	}

	// 2. Validate TIME
	timeVal := values[1]
	if timeVal == "" {
		errs = append(errs, ValidationError{
			Line:    lineNum,
			Column:  2,
			Field:   "time",
			Message: "time field is empty",
		})
	} else if v.timePattern != nil && !v.timePattern.MatchString(timeVal) {
		// Allow special time values
		if timeVal != "TIME_ALL_DAY" && timeVal != "TIME_ONGOING" {
			errs = append(errs, ValidationError{
				Line:    lineNum,
				Column:  2,
				Field:   "time",
				Value:   timeVal,
				Pattern: v.cfg.Crawler.Validation.Patterns.Time,
				Message: fmt.Sprintf("time '%s' does not match pattern", timeVal),
			})
		}
	}

	// 3. Validate DESCRIPTION/EVENT
	descVal := values[2]
	if descVal == "" {
		errs = append(errs, ValidationError{
			Line:    lineNum,
			Column:  3,
			Field:   "description",
			Message: "description/event field is empty",
		})
	} else if v.descriptionPattern != nil && !v.descriptionPattern.MatchString(descVal) {
		errs = append(errs, ValidationError{
			Line:    lineNum,
			Column:  3,
			Field:   "description",
			Value:   truncate(descVal, 50),
			Pattern: v.cfg.Crawler.Validation.Patterns.Description,
			Message: "description does not match pattern",
		})
	}

	// Validate casualties if required (check remaining columns for keywords)
	remaining := strings.Join(values[3:], " ")
	if v.cfg.Crawler.Validation.ValidateCasualties && v.casualtiesPattern != nil {
		if strings.Contains(remaining, "死") || strings.Contains(remaining, "傷") || strings.Contains(remaining, "失蹤") {
			// Check casualties pattern - currently simplified logic
			// TODO: Add validation error if pattern doesn't match
			_ = v.casualtiesPattern.MatchString(remaining)
		}
	}

	// 4. Validate END column (if present, usually index 8)
	// DATE | TIME | EVENT | CATEGORY | STATUS_NOTE | SOURCE | VIDEO | PHOTO | END
	if len(values) > 8 {
		endVal := strings.TrimSpace(values[8])
		if endVal != "" {
			validEnd := false
			validEnds := []string{"x", "X", "true", "TRUE"}
			for _, v := range validEnds {
				if endVal == v {
					validEnd = true
					break
				}
			}
			if !validEnd {
				errs = append(errs, ValidationError{
					Line:    lineNum,
					Column:  9,
					Field:   "end",
					Value:   endVal,
					Message: "invalid END column value (expected 'x' or empty)",
				})
			}
		}
	}

	return errs
}

// ValidateSingleRow validates a single parsed row (helper for during parsing).
func (v *MarkdownValidator) ValidateSingleRow(time, date, description string) error {
	if time == "" {
		return ErrTimeRequired
	}

	if description == "" {
		return ErrDescriptionRequired
	}

	if v.timePattern != nil && !v.timePattern.MatchString(time) {
		return fmt.Errorf("%w: %s", ErrInvalidTimeFormat, time)
	}

	if v.datePattern != nil && !v.datePattern.MatchString(date) {
		return fmt.Errorf("%w: %s", ErrInvalidDateFormat, date)
	}

	if v.descriptionPattern != nil && !v.descriptionPattern.MatchString(description) {
		return ErrDescriptionPattern
	}

	return nil
}

// extractDateFromHeader extracts date from markdown header format.
func extractDateFromHeader(header string) string {
	// Extract from **11月26日** format
	header = strings.TrimPrefix(header, "**")
	header = strings.TrimSuffix(header, "**")

	return header
}

// truncate truncates string to max length.
func truncate(s string, maxLen int) string {
	if len(s) > maxLen {
		return s[:maxLen] + "..."
	}

	return s
}

// String returns string representation of validation result.
func (r *ValidationResult) String() string {
	status := "✅ VALID"
	if !r.IsValid {
		status = "❌ INVALID"
	}

	return fmt.Sprintf(
		"%s | Total: %d | Valid: %d | Invalid: %d | Warnings: %d",
		status,
		r.Stats.TotalRows,
		r.Stats.ValidRows,
		r.Stats.InvalidRows,
		len(r.Warnings),
	)
}

// PrintErrors prints validation errors in readable format.
func (r *ValidationResult) PrintErrors() {
	if len(r.Errors) == 0 {
		return
	}

	fmt.Println("❌ Validation Errors:")

	for _, err := range r.Errors {
		if err.Line > 0 {
			fmt.Printf("  Line %d, Col %d", err.Line, err.Column)

			if err.Field != "" {
				fmt.Printf(" [%s]", err.Field)
			}

			fmt.Printf(": %s\n", err.Message)

			if err.Value != "" {
				fmt.Printf("    Found: %q\n", err.Value)
			}

			if err.Pattern != "" {
				fmt.Printf("    Expected pattern: %s\n", err.Pattern)
			}
		} else {
			fmt.Printf("  %s\n", err.Message)
		}
	}
}

// PrintWarnings prints validation warnings.
func (r *ValidationResult) PrintWarnings() {
	if len(r.Warnings) == 0 {
		return
	}

	fmt.Println("⚠️  Validation Warnings:")

	for _, warn := range r.Warnings {
		fmt.Printf("  %s\n", warn)
	}
}
