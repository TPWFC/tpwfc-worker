package integration

import (
	"os"
	"path/filepath"
	"testing"

	"tpwfc/internal/crawler"
	"tpwfc/internal/models"
	"tpwfc/internal/normalizer"
)

func TestWorkerFlow_StandardTimeline(t *testing.T) {
	// Path to fixture
	fixturePath := filepath.Join("..", "fixtures", "full_timeline.md")

	// Read fixture
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	// 1. Ingestion/Processing (Simulating 'worker' phases 1 & 2)
	parser := crawler.NewParser()

	doc, err := parser.ParseDocument(string(content))
	if err != nil {
		t.Fatalf("ParseDocument failed: %v", err)
	}

	// 2. Transformation (Normalizer)
	processor := normalizer.NewProcessor()

	result, err := processor.Process(doc)
	if err != nil {
		t.Fatalf("Process failed: %v", err)
	}

	timeline, ok := result.(*models.Timeline)
	if !ok {
		t.Fatalf("Expected *models.Timeline, got %T", result)
	}

	// 3. Verification (Simulating what would be uploaded)

	// Basic Info
	if timeline.BasicInfo.IncidentID != "TEST_FIRE_2025" {
		t.Errorf("Expected IncidentID TEST_FIRE_2025, got %s", timeline.BasicInfo.IncidentID)
	}

	// Events
	if len(timeline.Events) != 2 {
		t.Fatalf("Expected 2 timeline events, got %d", len(timeline.Events))
	}

	if timeline.Events[0].Description != "Event 1" {
		t.Errorf("Expected Event 1 description, got %s", timeline.Events[0].Description)
	}

	if timeline.Events[0].Casualties.Deaths != 1 {
		t.Errorf("Expected 1 death in Event 1, got %d", timeline.Events[0].Casualties.Deaths)
	}

	// Summary
	if timeline.Summary.TotalEvents != 2 {
		t.Errorf("Expected TotalEvents 2, got %d", timeline.Summary.TotalEvents)
	}
	// Deaths: KeyStats says 10, so we expect 10 (not aggregated from events which is 1)
	if timeline.Summary.TotalDeaths != 10 {
		t.Errorf("Expected TotalDeaths 10, got %d", timeline.Summary.TotalDeaths)
	}
	// Injured: Event 1 (0) + Event 2 (2) = 2
	if timeline.Summary.TotalInjured != 2 {
		t.Errorf("Expected TotalInjured 2, got %d", timeline.Summary.TotalInjured)
	}

	// Key Statistics
	if timeline.KeyStatistics.FinalDeaths != 10 {
		t.Errorf("Expected KeyStats FinalDeaths 10, got %d", timeline.KeyStatistics.FinalDeaths)
	}
}
