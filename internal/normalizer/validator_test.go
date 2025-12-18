package normalizer

import (
	"strings"
	"testing"

	"tpwfc/internal/models"
)

func TestNewValidator(t *testing.T) {
	v := NewValidator()
	if v == nil {
		t.Fatal("NewValidator returned nil")
	}
}

func TestValidator_Validate(t *testing.T) {
	v := NewValidator()

	validDoc := &models.TimelineDocument{
		BasicInfo: models.BasicInfo{
			IncidentID:   "test-id",
			IncidentName: "Test Incident",
		},
		Events: []models.TimelineEvent{
			{
				ID:       "event-1",
				Date:     "2023-01-01",
				Time:     "10:00",
				DateTime: "2023-01-01T10:00:00",
			},
		},
		Sources: []models.Source{
			{
				Name: "Source 1",
			},
		},
	}

	// Test valid document
	err := v.Validate(validDoc)
	if err != nil {
		t.Errorf("Validate returned unexpected error for valid doc: %v", err)
	}
}

func TestValidator_Validate_Errors(t *testing.T) {
	v := NewValidator()

	tests := []struct {
		name    string
		data    interface{}
		wantErr string
	}{
		{
			name:    "Nil input",
			data:    nil,
			wantErr: "invalid data type",
		},
		{
			name:    "Wrong type",
			data:    "string data",
			wantErr: "invalid data type",
		},
		{
			name: "Missing Incident ID",
			data: &models.TimelineDocument{
				BasicInfo: models.BasicInfo{IncidentName: "Test"},
			},
			wantErr: "missing incident ID",
		},
		{
			name: "Missing Incident Name",
			data: &models.TimelineDocument{
				BasicInfo: models.BasicInfo{IncidentID: "id"},
			},
			wantErr: "missing incident name",
		},
		{
			name: "No Events",
			data: &models.TimelineDocument{
				BasicInfo: models.BasicInfo{IncidentID: "id", IncidentName: "name"},
				Events:    []models.TimelineEvent{},
			},
			wantErr: "contains no events",
		},
		{
			name: "No Sources",
			data: &models.TimelineDocument{
				BasicInfo: models.BasicInfo{IncidentID: "id", IncidentName: "name"},
				Events: []models.TimelineEvent{
					{ID: "event-1", Date: "2023-01-01", Time: "10:00", DateTime: "2023-01-01T10:00:00"},
				},
				Sources: []models.Source{},
			},
			wantErr: "contains no sources",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := v.Validate(tt.data)
			if err == nil {
				t.Error("Validate expected error but got nil")
			} else if !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("Validate error = %v, want substring %v", err, tt.wantErr)
			}
		})
	}
}
