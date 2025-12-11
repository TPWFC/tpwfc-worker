package normalizer

import (
	"testing"

	"tpwfc/internal/models"
)

const testID = "test-id"

func TestNewProcessor(t *testing.T) {
	p := NewProcessor()
	if p == nil {
		t.Fatal("NewProcessor returned nil")
	}
}

func TestProcessor_Process(t *testing.T) {
	p := NewProcessor()

	validDoc := &models.TimelineDocument{
		BasicInfo: models.BasicInfo{
			IncidentID:   "test-id",
			IncidentName: "Test Incident",
		},
		Events: []models.TimelineEvent{
			{
				Date:     "2023-01-01",
				Time:     "10:00",
				DateTime: "2023-01-01T10:00:00",
			},
		},
		Sources: []models.Source{{Name: "Source 1"}},
	}

	result, err := p.Process(validDoc)
	if err != nil {
		t.Errorf("Process returned unexpected error: %v", err)
	}

	timeline, ok := result.(*models.Timeline)
	if !ok {
		t.Errorf("Process result is not *models.Timeline")
	} else if timeline.BasicInfo.IncidentID != testID {
		t.Errorf("ID = %s, want test-id", timeline.BasicInfo.IncidentID)
	}
}

func TestProcessor_Process_ValidationError(t *testing.T) {
	p := NewProcessor()

	invalidDoc := &models.TimelineDocument{
		BasicInfo: models.BasicInfo{IncidentID: ""}, // Missing ID
	}

	result, err := p.Process(invalidDoc)
	if err == nil {
		t.Error("Process expected error for invalid input")
	}

	if result != nil {
		t.Error("Process expected nil result for invalid input")
	}
}
