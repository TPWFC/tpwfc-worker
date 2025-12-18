package integration

import (
	"path/filepath"
	"testing"

	"tpwfc/internal/crawler"
	"tpwfc/internal/crawler/parsers"
)

const testEvent1Desc = "Event 1"

func TestCrawler_LocalFile(t *testing.T) {
	// Path to fixture
	fixturePath := filepath.Join("..", "fixtures", "full_timeline.md")

	// Initialize Crawler Components
	scraper := crawler.NewScraper()
	parser := parsers.NewParser()
	client := crawler.NewClientWithDeps(scraper, parser, nil)

	// Run Crawl (Simulating what 'crawler' cmd does with -file)
	events, err := client.CrawlTimelineFromFile(fixturePath)
	if err != nil {
		t.Fatalf("CrawlTimelineFromFile failed: %v", err)
	}

	// Verify Events
	if len(events) != 2 {
		t.Fatalf("Expected 2 events, got %d", len(events))
	}

	// Verify Event Content
	if events[0].Description != testEvent1Desc {
		t.Errorf("Expected first event description 'Event 1', got '%s'", events[0].Description)
	}

	if events[0].Sources[0].Name != "S1" {
		t.Errorf("Expected Source S1, got %s", events[0].Sources[0].Name)
	}
}
