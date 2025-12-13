package crawler

import (
	"encoding/json"
	"fmt"
	"os"

	"tpwfc/internal/models"
)

// Client manages HTTP communications and data flow for crawling.
type Client struct {
	scraper    *Scraper
	parser     *Parser
	urlManager *URLManager
}

// NewClient creates a new crawler client with default dependencies.
func NewClient() *Client {
	return &Client{
		scraper:    NewScraper(),
		parser:     NewParser(),
		urlManager: nil,
	}
}

// NewClientWithDeps creates a new crawler client with injected dependencies.
func NewClientWithDeps(scraper *Scraper, parser *Parser, urlManager *URLManager) *Client {
	return &Client{
		scraper:    scraper,
		parser:     parser,
		urlManager: urlManager,
	}
}

// CrawlTimeline fetches and parses a markdown timeline.
func (c *Client) CrawlTimeline(url string) ([]models.TimelineEvent, error) {
	// Fetch raw markdown
	content, err := c.scraper.Scrape(url)
	if err != nil {
		return nil, fmt.Errorf("failed to scrape URL: %w", err)
	}

	// Parse markdown table
	events, err := c.parser.ParseMarkdownTable(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse markdown: %w", err)
	}

	return events, nil
}

// CrawlTimelineFromFile reads and parses a markdown timeline from a local file.
func (c *Client) CrawlTimelineFromFile(filePath string) ([]models.TimelineEvent, error) {
	// Read local markdown file
	content, err := c.scraper.ReadLocalFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read local file: %w", err)
	}

	// Parse markdown table
	events, err := c.parser.ParseMarkdownTable(content)
	if err != nil {
		return nil, fmt.Errorf("failed to parse markdown: %w", err)
	}

	return events, nil
}

// CrawlTimelineFromFileWithMetrics returns (events, fileSize, duration, error).
func (c *Client) CrawlTimelineFromFileWithMetrics(filePath string) ([]models.TimelineEvent, int64, error) {
	// Read local markdown file with metrics
	content, fileSize, _, err := c.scraper.ReadLocalFileWithMetrics(filePath)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to read local file: %w", err)
	}

	// Parse markdown table
	events, err := c.parser.ParseMarkdownTable(content)
	if err != nil {
		return nil, 0, fmt.Errorf("failed to parse markdown: %w", err)
	}

	return events, fileSize, nil
}

// SaveTimelineJSON saves timeline events to JSON file.
func (c *Client) SaveTimelineJSON(events []models.TimelineEvent, outputPath string) error {
	// Calculate summary
	summary := calculateSummary(events)

	// Create output structure
	output := map[string]interface{}{
		"timeline": events,
		"summary":  summary,
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Write to file
	err = os.WriteFile(outputPath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// SaveTimelineJSONWithDocument saves timeline events with full document data to JSON file.
func (c *Client) SaveTimelineJSONWithDocument(events []models.TimelineEvent, doc *models.TimelineDocument, outputPath string) error {
	// Calculate summary
	summary := calculateSummary(events)

	// Create output structure with BasicInfo
	output := map[string]interface{}{
		"metadata":      doc.Metadata,
		"timeline":      events,
		"summary":       summary,
		"basicInfo":     doc.BasicInfo,
		"fireCause":     doc.FireCause,
		"severity":      doc.Severity,
		"keyStatistics": doc.KeyStatistics,
		"sources":       doc.Sources,
		"notes":         doc.Notes,
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Write to file
	err = os.WriteFile(outputPath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// SaveDetailedTimelineJSON saves detailed timeline data (phases, events, long-term tracking) to JSON file.
func (c *Client) SaveDetailedTimelineJSON(doc *models.DetailedTimelineDocument, outputPath string) error {
	// Create output structure
	output := map[string]interface{}{
		"phases":           doc.Phases,
		"longTermTracking": doc.LongTermTracking,
		"notes":            doc.Notes,
	}

	// Marshal to JSON
	jsonData, err := json.MarshalIndent(output, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal JSON: %w", err)
	}

	// Write to file
	err = os.WriteFile(outputPath, jsonData, 0644)
	if err != nil {
		return fmt.Errorf("failed to write file: %w", err)
	}

	return nil
}

// Get fetches a URL and returns the response (legacy).
func (c *Client) Get(url string) (string, error) {
	return c.scraper.Scrape(url)
}

// Helper function to calculate summary statistics.
func calculateSummary(events []models.TimelineEvent) map[string]interface{} {
	totalDeaths := 0
	totalInjured := 0
	totalMissing := 0
	startDate := ""
	endDate := ""

	for i, event := range events {
		totalDeaths += event.Casualties.Deaths
		totalInjured += event.Casualties.Injured
		totalMissing += event.Casualties.Missing

		if i == 0 {
			startDate = event.Date
		}

		endDate = event.Date
	}

	return map[string]interface{}{
		"startDate":    startDate,
		"endDate":      endDate,
		"totalEvents":  len(events),
		"totalDeaths":  totalDeaths,
		"totalInjured": totalInjured,
		"totalMissing": totalMissing,
	}
}
