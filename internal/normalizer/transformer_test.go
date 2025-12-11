package normalizer

import (
	"testing"

	"tpwfc/internal/models"
)

const testIncidentName = "Test Incident"

func TestNewTransformer(t *testing.T) {
	tr := NewTransformer()
	if tr == nil {
		t.Fatal("NewTransformer returned nil")
	}
}

func TestTransformer_Transform(t *testing.T) {
	tr := NewTransformer()

	inputDoc := &models.TimelineDocument{
		BasicInfo: models.BasicInfo{
			IncidentID:   "test-id",
			IncidentName: "Test Incident",
			Location:     "Test Location",
			StartDate:    "2023-01-01",
			EndDate:      "2023-01-02",
			DateRange:    "2023-01-01 to 2023-01-02",
		},
		Events: []models.TimelineEvent{
			{
				Casualties: models.CasualtyData{Injured: 5},
			},
			{
				Casualties: models.CasualtyData{Injured: 3},
			},
		},
		KeyStatistics: models.KeyStatistics{
			FinalDeaths:    "10 deaths",
			MissingPersons: 2,
		},
		Sources: []models.Source{{Name: "Source 1"}},
	}

	result, err := tr.Transform(inputDoc)
	if err != nil {
		t.Fatalf("Transform returned unexpected error: %v", err)
	}

	timeline, ok := result.(*models.Timeline)
	if !ok {
		t.Fatalf("Transform result is not *models.Timeline")
	}

	// Verify mappings
	if timeline.BasicInfo.IncidentID != "test-id" {
		t.Errorf("ID = %s, want test-id", timeline.BasicInfo.IncidentID)
	}

	if timeline.BasicInfo.IncidentName != testIncidentName {
		t.Errorf("Title = %s, want Test Incident", timeline.BasicInfo.IncidentName)
	}

	if timeline.BasicInfo.Location != "Test Location" {
		t.Errorf("Location = %s, want Test Location", timeline.BasicInfo.Location)
	}

	// Verify Summary
	summary := timeline.Summary
	if summary.Title != "Test Incident" {
		t.Errorf("Summary.Title = %s, want Test Incident", summary.Title)
	}

	if summary.TotalEvents != 2 {
		t.Errorf("Summary.TotalEvents = %d, want 2", summary.TotalEvents)
	}

	if summary.TotalDeaths != 10 {
		t.Errorf("Summary.TotalDeaths = %d, want 10", summary.TotalDeaths)
	}

	if summary.TotalMissing != 2 {
		t.Errorf("Summary.TotalMissing = %d, want 2", summary.TotalMissing)
	}

	if summary.TotalInjured != 8 {
		t.Errorf("Summary.TotalInjured = %d, want 8", summary.TotalInjured)
	}
}

func TestTransformer_Transform_Error(t *testing.T) {
	tr := NewTransformer()

	_, err := tr.Transform("invalid input")
	if err == nil {
		t.Error("Transform expected error for invalid input")
	}
}
