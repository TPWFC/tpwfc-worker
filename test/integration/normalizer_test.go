package integration

import (
	"os"
	"path/filepath"
	"testing"

	"tpwfc/internal/crawler/parsers"
)

func TestNormalizer_DetailedTimeline(t *testing.T) {
	// Path to fixture
	fixturePath := filepath.Join("..", "fixtures", "detailed_timeline.md")

	// Read fixture
	content, err := os.ReadFile(fixturePath)
	if err != nil {
		t.Fatalf("Failed to read fixture: %v", err)
	}

	// Initialize Parser
	parser := parsers.NewParser()

	// Parse (Simulating 'normalizer' logic)
	doc, err := parser.ParseDetailedTimeline(string(content))
	if err != nil {
		t.Fatalf("ParseDetailedTimeline failed: %v", err)
	}

	// Verify Phases
	if len(doc.Phases) != 1 {
		t.Fatalf("Expected 1 phase, got %d", len(doc.Phases))
	}

	phase := doc.Phases[0]
	if phase.PhaseName != "Phase 1" {
		t.Errorf("Expected PhaseName 'Phase 1', got '%s'", phase.PhaseName)
	}

	if len(phase.Events) != 1 {
		t.Errorf("Expected 1 event in phase, got %d", len(phase.Events))
	}

	if phase.Events[0].Event != "Detailed Event 1" {
		t.Errorf("Expected event description 'Detailed Event 1', got '%s'", phase.Events[0].Event)
	}

	// Verify Long Term Tracking
	if len(doc.LongTermTracking) != 1 {
		t.Errorf("Expected 1 tracking event, got %d", len(doc.LongTermTracking))
	}

	if doc.LongTermTracking[0].Event != "Tracking Event" {
		t.Errorf("Expected tracking event 'Tracking Event', got '%s'", doc.LongTermTracking[0].Event)
	}
}
