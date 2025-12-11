package utils

import "strings"

// StringHelper provides string utility functions.
type StringHelper struct{}

// NewStringHelper creates a new string helper.
func NewStringHelper() *StringHelper {
	return &StringHelper{}
}

// TrimWhitespace removes leading and trailing whitespace.
func (s *StringHelper) TrimWhitespace(str string) string {
	return strings.TrimSpace(str)
}

// NormalizeWhitespace replaces multiple whitespace with single space.
func (s *StringHelper) NormalizeWhitespace(str string) string {
	return strings.Join(strings.Fields(str), " ")
}

// TruncateString truncates string to max length.
func (s *StringHelper) TruncateString(str string, maxLength int) string {
	if len(str) <= maxLength {
		return str
	}

	return str[:maxLength] + "..."
}
