// Package utils provides common utility functions.
package utils

import "net/http"

// HTTPHelper provides HTTP utility functions.
type HTTPHelper struct{}

// NewHTTPHelper creates a new HTTP helper.
func NewHTTPHelper() *HTTPHelper {
	return &HTTPHelper{}
}

// IsValidURL checks if a URL is valid.
func (h *HTTPHelper) IsValidURL(url string) bool {
	// TODO: Implement URL validation
	return true
}

// BuildHeaders creates HTTP headers with defaults.
func (h *HTTPHelper) BuildHeaders(customHeaders map[string]string) http.Header {
	headers := http.Header{}

	// Add default headers
	headers.Add("User-Agent", "TPWFC-Worker/1.0")
	headers.Add("Accept", "application/json, text/html")

	// Add custom headers
	for key, value := range customHeaders {
		headers.Add(key, value)
	}

	return headers
}
