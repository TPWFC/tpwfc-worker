// Package validator provides validation utilities for markdown documents.
package validator

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"regexp"
	"strings"

	"tpwfc/internal/config"
)

// Validation errors.
var (
	ErrTimeRequired        = errors.New("time is required")
	ErrDescriptionRequired = errors.New("description is required")
	ErrInvalidTimeFormat   = errors.New("invalid time format")
	ErrInvalidDateFormat   = errors.New("invalid date format")
	ErrDescriptionPattern  = errors.New("description does not match pattern")
	ErrLintFailed          = errors.New("markdown linting failed")
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
	var colMap map[string]int

	for lineNum, line := range lines {
		line = strings.TrimSpace(line)

		// Skip empty lines
		if line == "" {
			tableStarted = false
			continue
		}

		// Skip markdown table separators
		if strings.HasPrefix(line, "|") && (strings.Contains(line, "---") || strings.Contains(line, ":---")) {
			continue
		}

		// Check for Table Rows
		if strings.HasPrefix(line, "|") {
			// Split and clean cells
			cells := strings.Split(line, "|")
			var cleanCells []string
			cleanCells = append(cleanCells, cells...)
			if len(cleanCells) > 0 && strings.TrimSpace(cleanCells[0]) == "" {
				cleanCells = cleanCells[1:]
			}
			if len(cleanCells) > 0 && strings.TrimSpace(cleanCells[len(cleanCells)-1]) == "" {
				cleanCells = cleanCells[:len(cleanCells)-1]
			}

			// Check if this is a header row (Timeline table specific)
			upperLine := strings.ToUpper(line)
			isTimelineHeader := strings.Contains(upperLine, "DATE") &&
				strings.Contains(upperLine, "TIME") &&
				strings.Contains(upperLine, "EVENT")

			if isTimelineHeader {
				tableStarted = true
				colMap = make(map[string]int)
				for idx, cell := range cleanCells {
					// Import NormalizeHeader from parsers if possible, but cyclic dependency might prevent it.
					// We'll reimplement simple normalization or just string matching here since this is a validator.
					// Ideally we should move NormalizeHeader to a shared package, but for now we'll duplicate simple logic.
					h := strings.ToUpper(strings.TrimSpace(cell))
					if strings.Contains(h, "DATE") {
						colMap["DATE"] = idx
					} else if strings.Contains(h, "TIME") {
						colMap["TIME"] = idx
					} else if strings.Contains(h, "EVENT") || strings.Contains(h, "DESCRIPTION") {
						colMap["EVENT"] = idx
					} else if strings.Contains(h, "END") {
						colMap["END"] = idx
					}
				}
				continue
			}

			// If we are in a table but it's not the timeline table (e.g. KEY|VALUE), skip validation or implement generic validation
			if !tableStarted {
				continue
			}

			// Process table data rows
			rowNumber++
			result.Stats.TotalRows++

			rowError := v.validateRow(cleanCells, lineNum+1, colMap)
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
			colMap = nil
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

// validateRow validates a single table row.
func (v *MarkdownValidator) validateRow(values []string, lineNum int, colMap map[string]int) []ValidationError {
	var errs []ValidationError

	// Expect: DATE | TIME | DESCRIPTION at minimum
	// If we have a colMap, we check for those indices.
	// If no colMap (shouldn't happen if logic above is correct), fall back or error.

	getCell := func(key string) (string, bool) {
		if idx, ok := colMap[key]; ok && idx < len(values) {
			return strings.TrimSpace(values[idx]), true
		}
		return "", false
	}

	// 1. Validate DATE (YYYY-MM-DD)
	dateVal, hasDate := getCell("DATE")
	dateRegex := regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

	if hasDate {
		if dateVal == "" {
			errs = append(errs, ValidationError{
				Line:    lineNum,
				Column:  colMap["DATE"] + 1,
				Field:   "date",
				Message: "date field is empty",
			})
		} else if !dateRegex.MatchString(dateVal) {
			errs = append(errs, ValidationError{
				Line:    lineNum,
				Column:  colMap["DATE"] + 1,
				Field:   "date",
				Value:   dateVal,
				Pattern: "YYYY-MM-DD",
				Message: fmt.Sprintf("date '%s' invalid format", dateVal),
			})
		}
	} else {
		// Only report missing date if we expected a date column but didn't find it in the map?
		// Actually if the header didn't have DATE, maybe we shouldn't fail row validation?
		// But for Timeline, DATE is mandatory.
		errs = append(errs, ValidationError{
			Line:    lineNum,
			Message: "missing DATE column",
		})
	}

	// 2. Validate TIME
	timeVal, hasTime := getCell("TIME")
	if hasTime {
		if timeVal == "" {
			errs = append(errs, ValidationError{
				Line:    lineNum,
				Column:  colMap["TIME"] + 1,
				Field:   "time",
				Message: "time field is empty",
			})
		} else if v.timePattern != nil && !v.timePattern.MatchString(timeVal) {
			// Allow special time values
			if timeVal != "TIME_ALL_DAY" && timeVal != "TIME_ONGOING" {
				errs = append(errs, ValidationError{
					Line:    lineNum,
					Column:  colMap["TIME"] + 1,
					Field:   "time",
					Value:   timeVal,
					Pattern: v.cfg.Crawler.Validation.Patterns.Time,
					Message: fmt.Sprintf("time '%s' does not match pattern", timeVal),
				})
			}
		}
	} else {
		errs = append(errs, ValidationError{
			Line:    lineNum,
			Message: "missing TIME column",
		})
	}

	// 3. Validate DESCRIPTION/EVENT
	descVal, hasDesc := getCell("EVENT")
	if hasDesc {
		if descVal == "" {
			errs = append(errs, ValidationError{
				Line:    lineNum,
				Column:  colMap["EVENT"] + 1,
				Field:   "description",
				Message: "description/event field is empty",
			})
		} else if v.descriptionPattern != nil && !v.descriptionPattern.MatchString(descVal) {
			errs = append(errs, ValidationError{
				Line:    lineNum,
				Column:  colMap["EVENT"] + 1,
				Field:   "description",
				Value:   truncate(descVal, 50),
				Pattern: v.cfg.Crawler.Validation.Patterns.Description,
				Message: "description does not match pattern",
			})
		}
	}

	// 4. Validate END column (if present)
	if idx, ok := colMap["END"]; ok && idx < len(values) {
		endVal := strings.TrimSpace(values[idx])
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
					Column:  idx + 1,
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

// Lint checks the markdown formatting using deno fmt --check.
func (v *MarkdownValidator) Lint(filePath string) error {
	cmd := exec.Command("deno", "fmt", "--check", filePath)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	if err != nil {
		// Deno fmt --check returns non-zero exit code if formatting issues are found
		combinedOutput := stderr.String()
		if combinedOutput == "" {
			combinedOutput = stdout.String()
		}
		return fmt.Errorf("%w:\n%s", ErrLintFailed, combinedOutput)
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
