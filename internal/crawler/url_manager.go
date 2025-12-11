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
	}
}

// NextURL returns the next URL to try. For local files, url will be the file path.
func (um *URLManager) NextURL() (string, string, string, string, int, error) {
	if len(um.sources) == 0 {
		return "", "", "", "", 0, ErrNoSourcesAvailable
	}

	source := um.sources[um.currentSourceIdx]
	sourceKey := source.FireID + ":" + source.Language
	attemptNum := um.sourceAttempts[sourceKey] + 1

	// For local files, only one attempt is needed (no retry)
	if source.IsLocalFile() {
		if attemptNum > 1 {
			// Move to next source
			um.currentSourceIdx++
			um.currentURLIdx = 0
			um.sourceAttempts[sourceKey] = 0

			if um.currentSourceIdx >= len(um.sources) {
				return "", "", "", "", 0, fmt.Errorf("%w: %d", ErrAllSourcesExhausted, len(um.sources))
			}

			source = um.sources[um.currentSourceIdx]
			sourceKey = source.FireID + ":" + source.Language

			// Check if next source is also local file
			if source.IsLocalFile() {
				um.sourceAttempts[sourceKey] = 1

				return source.File, source.Name, source.FireID, source.Language, 1, nil
			}
		}

		um.sourceAttempts[sourceKey] = 1

		return source.File, source.Name, source.FireID, source.Language, 1, nil
	}

	// Check if we've exhausted retry attempts for this source
	if attemptNum > um.retryPolicy.MaxAttempts {
		// Move to next source
		um.currentSourceIdx++
		um.currentURLIdx = 0
		um.sourceAttempts[sourceKey] = 0

		if um.currentSourceIdx >= len(um.sources) {
			return "", "", "", "", 0, fmt.Errorf("%w: %d", ErrAllSourcesExhausted, len(um.sources))
		}

		source = um.sources[um.currentSourceIdx]
		sourceKey = source.FireID + ":" + source.Language

		// Check if next source is local file
		if source.IsLocalFile() {
			um.sourceAttempts[sourceKey] = 1

			return source.File, source.Name, source.FireID, source.Language, 1, nil
		}

		um.sourceAttempts[sourceKey] = 1

		return source.URL, source.Name, source.FireID, source.Language, 1, nil
	}

	// Try next URL variant (primary or backup)
	allURLs := source.GetAllURLs()
	if um.currentURLIdx >= len(allURLs) {
		um.currentURLIdx = 0
		um.sourceAttempts[sourceKey] = attemptNum
	}

	url := allURLs[um.currentURLIdx]
	um.currentURLIdx++

	return url, source.Name, source.FireID, source.Language, attemptNum, nil
}

// IsCurrentSourceLocal returns true if the current source is a local file.
func (um *URLManager) IsCurrentSourceLocal() bool {
	if um.currentSourceIdx < len(um.sources) {
		return um.sources[um.currentSourceIdx].IsLocalFile()
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
	um.sourceAttempts = make(map[string]int)
	um.attemptLog = make(map[string][]AttempResult)
}
