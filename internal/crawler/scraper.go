package crawler

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"

	"tpwfc/internal/config"
)

// ErrUnexpectedStatusCode indicates an HTTP response with unexpected status.
var ErrUnexpectedStatusCode = errors.New("unexpected status code")

// Scraper handles web scraping operations with config-driven retry logic.
type Scraper struct {
	client       *http.Client
	retryPolicy  *config.RetryPolicy
	bufferSizeKb int
}

// NewScraper creates a new scraper instance with default config.
func NewScraper() *Scraper {
	return &Scraper{
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
		retryPolicy: &config.RetryPolicy{
			MaxAttempts:       3,
			InitialDelayMs:    500,
			MaxDelayMs:        30000,
			BackoffMultiplier: 2.0,
			TimeoutSec:        30,
		},
		bufferSizeKb: 1024,
	}
}

// NewScraperWithConfig creates a new scraper with custom retry policy.
func NewScraperWithConfig(retryPolicy *config.RetryPolicy, bufferSizeKb int) *Scraper {
	timeout := time.Duration(retryPolicy.TimeoutSec) * time.Second

	return &Scraper{
		client: &http.Client{
			Timeout: timeout,
		},
		retryPolicy:  retryPolicy,
		bufferSizeKb: bufferSizeKb,
	}
}

// ScrapeWithMetrics returns (content, statusCode, duration, error).
func (s *Scraper) ScrapeWithMetrics(url string) (string, int, time.Duration, error) {
	var lastErr error

	var lastStatusCode int

	totalDuration := time.Duration(0)

	for attempt := 1; attempt <= s.retryPolicy.MaxAttempts; attempt++ {
		startTime := time.Now()

		req, err := http.NewRequest(http.MethodGet, url, http.NoBody)
		if err != nil {
			lastErr = fmt.Errorf("failed to create request: %w", err)

			continue
		}

		// Set user agent to avoid being blocked
		req.Header.Set("User-Agent", "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/91.0.4472.124 Safari/537.36")
		req.Header.Set("Accept", "text/html,application/xhtml+xml,application/xml;q=0.9,*/*;q=0.8")

		resp, err := s.client.Do(req)
		duration := time.Since(startTime)
		totalDuration += duration

		if err != nil {
			lastErr = fmt.Errorf("request failed (attempt %d/%d): %w", attempt, s.retryPolicy.MaxAttempts, err)

			// Calculate backoff delay
			if attempt < s.retryPolicy.MaxAttempts {
				delay := s.retryPolicy.GetRetryDelay(attempt)
				if delay > 0 {
					time.Sleep(delay)
				}
			}

			continue
		}

		defer func() {
			if closeErr := resp.Body.Close(); closeErr != nil {
				lastErr = fmt.Errorf("failed to close response body: %w", closeErr)
			}
		}()
		lastStatusCode = resp.StatusCode

		if resp.StatusCode != http.StatusOK {
			lastErr = fmt.Errorf("%w: %d", ErrUnexpectedStatusCode, resp.StatusCode)

			// Only retry on specific status codes
			if attempt < s.retryPolicy.MaxAttempts && isRetryableStatus(resp.StatusCode) {
				delay := s.retryPolicy.GetRetryDelay(attempt)
				if delay > 0 {
					time.Sleep(delay)
				}
			}

			continue
		}

		// Read with buffer limit
		// bufferSizeKb is in KB, convert to bytes
		limit := int64(s.bufferSizeKb) * 1024
		reader := io.LimitReader(resp.Body, limit)

		body, err := io.ReadAll(reader)
		if err != nil {
			lastErr = fmt.Errorf("failed to read response body: %w", err)

			continue
		}

		return string(body), resp.StatusCode, totalDuration, nil
	}

	return "", lastStatusCode, totalDuration, lastErr
}

// Scrape fetches and returns content from the given URL (legacy method).
func (s *Scraper) Scrape(url string) (string, error) {
	content, _, _, err := s.ScrapeWithMetrics(url)

	return content, err
}

// ReadLocalFile reads content from a local file path.
func (s *Scraper) ReadLocalFile(filePath string) (string, error) {
	content, err := os.ReadFile(filePath)
	if err != nil {
		return "", fmt.Errorf("failed to read local file %s: %w", filePath, err)
	}

	return string(content), nil
}

// ReadLocalFileWithMetrics returns (content, fileSize, duration, error).
func (s *Scraper) ReadLocalFileWithMetrics(filePath string) (string, int64, time.Duration, error) {
	startTime := time.Now()

	fileInfo, err := os.Stat(filePath)
	if err != nil {
		return "", 0, time.Since(startTime), fmt.Errorf("failed to stat file %s: %w", filePath, err)
	}

	content, err := os.ReadFile(filePath)
	duration := time.Since(startTime)

	if err != nil {
		return "", 0, duration, fmt.Errorf("failed to read local file %s: %w", filePath, err)
	}

	return string(content), fileInfo.Size(), duration, nil
}

// isRetryableStatus determines if we should retry based on HTTP status code.
func isRetryableStatus(statusCode int) bool {
	// Retry on temporary failures
	switch statusCode {
	case http.StatusServiceUnavailable: // 503
		return true
	case http.StatusGatewayTimeout: // 504
		return true
	case http.StatusTooManyRequests: // 429
		return true
	case http.StatusRequestTimeout: // 408
		return true
	}

	return false
}
