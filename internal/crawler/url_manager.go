package crawler

import (
	"errors"
	"fmt"
	"time"

	"tpwfc/internal/config"
	"tpwfc/internal/logger"
)

// URL manager errors.
var (
	ErrNoSourcesAvailable  = errors.New("no sources available")
	ErrAllSourcesExhausted = errors.New("all sources exhausted")
)

// URLManager manages multiple source URLs with fallback logic and backup URLs.
type URLManager struct {
	retryPolicy      *config.RetryPolicy
	attemptLog       map[string][]AttempResult
	sourceAttempts   map[string]int
	sources          []config.SourceConfig
	currentSourceIdx int
	currentURLIdx    int
	// isFallbackMode indicates if we are currently falling back to local file for the current source
	isFallbackMode bool
}

// AttempResult records the result of a URL fetch attempt.
type AttempResult struct {
	Timestamp  time.Time
	URL        string
	Error      string
	Attempt    int
	Duration   time.Duration
	StatusCode int
	Success    bool
}

// SourceInfo holds information about a source's current state.
type SourceInfo struct {
	FireID   string
	FireName string
	Language string
	URL      string
	Name     string
}

// NewURLManager creates a new URL manager.
func NewURLManager(cfg *config.Config) *URLManager {
	return &URLManager{
		sources:          cfg.GetEnabledSources(),
		retryPolicy:      &cfg.Crawler.Retry,
		attemptLog:       make(map[string][]AttempResult),
		sourceAttempts:   make(map[string]int),
		currentSourceIdx: 0,
		currentURLIdx:    0,
		isFallbackMode:   false,
	}
}

// NextURL returns the next URL to try. For local files, url will be the file path.
func (um *URLManager) NextURL() (string, string, string, string, int, error) {
	if len(um.sources) == 0 {
		return "", "", "", "", 0, ErrNoSourcesAvailable
	}

	// Check if current index is out of bounds
	if um.currentSourceIdx >= len(um.sources) {
		return "", "", "", "", 0, fmt.Errorf("%w: %d", ErrAllSourcesExhausted, len(um.sources))
	}

	source := um.sources[um.currentSourceIdx]
	sourceKey := source.FireID + ":" + source.Language

	// If in fallback mode, we are trying the local file
	if um.isFallbackMode {
		attemptNum := um.sourceAttempts[sourceKey] + 1

		// We only try the local file once in fallback mode
		if attemptNum > 1 {
			// Fallback failed or completed, move to next source
			return um.moveToNextSource()
		}

		um.sourceAttempts[sourceKey] = attemptNum
		return source.File, source.Name, source.FireID, source.Language, 1, nil
	}

	// Calculate attempt number for current phase (URL phase)
	attemptNum := um.sourceAttempts[sourceKey] + 1

	// For pure local files (no URL configured), treat as special case
	if source.URL == "" && source.IsLocalFile() {
		if attemptNum > 1 {
			return um.moveToNextSource()
		}

		um.sourceAttempts[sourceKey] = 1
		// For pure local files, we set fallback mode effectively to true for IsCurrentSourceLocal logic
		// or we can handle it by returning true in IsCurrentSourceLocal if URL is empty
		return source.File, source.Name, source.FireID, source.Language, 1, nil
	}

	// Check if we've exhausted retry attempts for the URL
	if attemptNum > um.retryPolicy.MaxAttempts {
		// Retries exhausted. Check if we can fallback to local file
		if source.IsLocalFile() {
			// Switch to fallback mode
			um.isFallbackMode = true
			um.sourceAttempts[sourceKey] = 0 // Reset attempts for fallback phase

			// Recursively call to get the local file
			return um.NextURL()
		}

		// No fallback available, move to next source
		return um.moveToNextSource()
	}

	// Try next URL variant (primary or backup)
	allURLs := source.GetAllURLs()
	if um.currentURLIdx >= len(allURLs) {
		um.currentURLIdx = 0
		// If we wrapped around URLs, that counts as a full "attempt" cycle in some logics,
		// but here we count strict attempts.
		// We'll keep using the same attempt count logic as before.
	}

	url := allURLs[um.currentURLIdx]
	um.currentURLIdx++ // Rotate through URLs for next call

	um.sourceAttempts[sourceKey] = attemptNum

	return url, source.Name, source.FireID, source.Language, attemptNum, nil
}

// moveToNextSource advances to the next source and resets state.
func (um *URLManager) moveToNextSource() (string, string, string, string, int, error) {
	um.currentSourceIdx++
	um.currentURLIdx = 0
	um.isFallbackMode = false

	// Clear attempt counts for the new source (optional, but good for cleanliness)
	// We don't strictly need to clear sourceAttempts[newKey] if we assume it starts at 0,
	// but keeping the map clean is okay. The map is persistent though.

	if um.currentSourceIdx >= len(um.sources) {
		return "", "", "", "", 0, fmt.Errorf("%w: %d", ErrAllSourcesExhausted, len(um.sources))
	}

	// Recursively call NextURL for the new source
	return um.NextURL()
}

// IsCurrentSourceLocal returns true if the current source is a local file.
func (um *URLManager) IsCurrentSourceLocal() bool {
	if um.currentSourceIdx < len(um.sources) {
		source := um.sources[um.currentSourceIdx]
		// It's local if we are in fallback mode OR if there is no URL (pure local source)
		return um.isFallbackMode || source.URL == ""
	}

	return false
}

// RecordAttempt records the result of a fetch attempt.
func (um *URLManager) RecordAttempt(url string, success bool, err error, statusCode int, duration time.Duration) {
	if um.attemptLog[url] == nil {
		um.attemptLog[url] = []AttempResult{}
	}

	errMsg := ""
	if err != nil {
		errMsg = err.Error()
	}

	um.attemptLog[url] = append(um.attemptLog[url], AttempResult{
		URL:        url,
		Attempt:    len(um.attemptLog[url]) + 1,
		Success:    success,
		Error:      errMsg,
		Timestamp:  time.Now(),
		Duration:   duration,
		StatusCode: statusCode,
	})
}

// GetRetryDelay returns the delay before next attempt.
func (um *URLManager) GetRetryDelay(attemptNum int) time.Duration {
	return um.retryPolicy.GetRetryDelay(attemptNum)
}

// HasMoreSources returns true if there are more sources to try.
func (um *URLManager) HasMoreSources() bool {
	return um.currentSourceIdx < len(um.sources)
}

// GetCurrentIndex returns the current source index.
func (um *URLManager) GetCurrentIndex() int {
	return um.currentSourceIdx
}

// GetSourceCount returns the total number of sources.
func (um *URLManager) GetSourceCount() int {
	return len(um.sources)
}

// GetCurrentSource returns the current source.
func (um *URLManager) GetCurrentSource() config.SourceConfig {
	if um.currentSourceIdx < len(um.sources) {
		return um.sources[um.currentSourceIdx]
	}

	return config.SourceConfig{}
}

// GetAttemptLog returns the attempt log for a URL.
func (um *URLManager) GetAttemptLog(url string) []AttempResult {
	return um.attemptLog[url]
}

// GetAttemptStats returns statistics about fetch attempts.
func (um *URLManager) GetAttemptStats() AttempStats {
	stats := AttempStats{
		TotalURLs:          len(um.sources),
		URLAttempts:        make(map[string]int),
		SuccessfulURLs:     0,
		FailedURLs:         0,
		TotalAttempts:      0,
		SuccessfulAttempts: 0,
		FailedAttempts:     0,
	}

	for url, results := range um.attemptLog {
		stats.URLAttempts[url] = len(results)
		stats.TotalAttempts += len(results)

		urlSuccess := false

		for _, result := range results {
			if result.Success {
				stats.SuccessfulAttempts++
				urlSuccess = true
			} else {
				stats.FailedAttempts++
			}
		}

		if urlSuccess {
			stats.SuccessfulURLs++
		} else {
			stats.FailedURLs++
		}
	}

	return stats
}

// AttempStats contains statistics about fetch attempts.
type AttempStats struct {
	URLAttempts        map[string]int
	TotalURLs          int
	SuccessfulURLs     int
	FailedURLs         int
	TotalAttempts      int
	SuccessfulAttempts int
	FailedAttempts     int
}

// String returns a string representation of attempt stats.
func (s AttempStats) String() string {
	return fmt.Sprintf(
		"URLs: %d total, %d success, %d failed | Attempts: %d total, %d success, %d failed",
		s.TotalURLs,
		s.SuccessfulURLs,
		s.FailedURLs,
		s.TotalAttempts,
		s.SuccessfulAttempts,
		s.FailedAttempts,
	)
}

// LogAttemptSummary logs a summary of fetch attempts using the provided logger.
func (um *URLManager) LogAttemptSummary(l *logger.Logger) {
	l.Info("📊 Fetch Attempt Summary:")

	for i, source := range um.sources {
		results := um.attemptLog[source.URL]

		l.Info(fmt.Sprintf("%d. %s", i+1, source.Name))
		l.Info(fmt.Sprintf("   URL: %s", source.URL))

		if len(results) == 0 {
			l.Info("   Status: Not attempted")
		} else {
			lastResult := results[len(results)-1]
			statusEmoji := "❌"

			if lastResult.Success {
				statusEmoji = "✅"
			}

			l.Info(fmt.Sprintf("   Status: %s (%d attempts)", statusEmoji, len(results)))

			for j, result := range results {
				statusStr := "✅ Success"
				if !result.Success {
					statusStr = fmt.Sprintf("❌ Failed: %s", result.Error)
				}

				l.Info(fmt.Sprintf("     Attempt %d: %s (%.2fs)", j+1, statusStr, result.Duration.Seconds()))
			}
		}
	}

	stats := um.GetAttemptStats()
	l.Info(fmt.Sprintf("Overall: %s", stats))
}

// Reset resets the URL manager state.
func (um *URLManager) Reset() {
	um.currentSourceIdx = 0
	um.currentURLIdx = 0
	um.isFallbackMode = false
	um.sourceAttempts = make(map[string]int)
	um.attemptLog = make(map[string][]AttempResult)
}
