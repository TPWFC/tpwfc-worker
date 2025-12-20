package payload

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"tpwfc/internal/logger"
	"tpwfc/internal/models"
)

var (
	ErrUnexpectedQuery  = errors.New("unexpected query")
	ErrWrongCredentials = errors.New("wrong credentials")
)

// MockClient implements the Client interface for testing.
type MockClient struct {
	ExecuteFunc func(query string, variables map[string]interface{}) (*GraphQLResponse, error)
	LoginFunc   func(email, password string) error
}

func (m *MockClient) Execute(query string, variables map[string]interface{}) (*GraphQLResponse, error) {
	if m.ExecuteFunc != nil {
		return m.ExecuteFunc(query, variables)
	}

	return nil, nil
}

func (m *MockClient) Login(email, password string) error {
	if m.LoginFunc != nil {
		return m.LoginFunc(email, password)
	}

	return nil
}

func TestUploader_Upload_Scenario(t *testing.T) {
	// 1. Setup Mock Client
	// We want to simulate:
	// - FindIncident (returns nil -> create new)
	// - CreateIncident (returns ID 100)
	// - Upload 1 Event (Find -> nil, Create -> success)
	mockClient := &MockClient{
		ExecuteFunc: func(query string, variables map[string]interface{}) (*GraphQLResponse, error) {
			// A. Find Fire Incident
			if query == FindFireIncidentQuery {
				// Return empty list (incident not found)
				return &GraphQLResponse{
					Data: json.RawMessage(`{"FireIncidents": {"docs": []}}`),
				}, nil
			}

			// B. Create Fire Incident
			if query == CreateFireIncidentMutation {
				// Return new ID 100
				return &GraphQLResponse{
					Data: json.RawMessage(`{"createFireIncident": {"id": 100}}`),
				}, nil
			}

			// C. Find Event
			if query == FindFireEventQuery {
				return &GraphQLResponse{
					Data: json.RawMessage(`{"FireEvents": {"docs": []}}`),
				}, nil
			}

			// D. Create Event
			if query == CreateFireEventMutation {
				return &GraphQLResponse{
					Data: json.RawMessage(`{"createFireEvent": {"id": 500}}`),
				}, nil
			}

			return nil, fmt.Errorf("%w: %s", ErrUnexpectedQuery, query)
		},
	}

	// 2. Setup Uploader with Mock
	mockLogger := logger.NewLogger("error") // suppress output
	uploader := NewUploaderWithClient(mockClient, mockLogger)

	// 3. Prepare Test Data
	data := &models.Timeline{
		Summary: models.TimelineSummary{
			TotalEvents: 1,
			StartDate:   "2025-01-01",
		},
		BasicInfo: models.BasicInfo{
			IncidentID:   "test-fire-id",
			IncidentName: "Test Fire",
			Map: models.MapSource{
				Name: "Test Map",
				URL:  "https://example.com/map",
			},
		},
		Events: []models.TimelineEvent{
			{
				ID:          "ev1",
				Date:        "2025-01-01",
				Description: "Event 1",
			},
		},
	}

	// 4. Run Upload
	result, err := uploader.Upload(data, "en")
	if err != nil {
		t.Fatalf("Upload failed: %v", err)
	}

	// 5. Assertions
	if result.IncidentID != 100 {
		t.Errorf("Expected IncidentID 100, got %d", result.IncidentID)
	}

	if result.EventsCreated != 1 {
		t.Errorf("Expected 1 event created, got %d", result.EventsCreated)
	}

	if result.EventsUpdated != 0 {
		t.Errorf("Expected 0 events updated, got %d", result.EventsUpdated)
	}

	if len(result.Errors) > 0 {
		t.Errorf("Expected no errors, got %d", len(result.Errors))
	}
}

func TestUploader_Authenticate(t *testing.T) {
	called := false
	mockClient := &MockClient{
		LoginFunc: func(email, password string) error {
			called = true
			if email != "admin@test.com" || password != "pass" {
				return ErrWrongCredentials
			}

			return nil
		},
	}

	uploader := NewUploaderWithClient(mockClient, logger.NewLogger("error"))
	err := uploader.Authenticate("admin@test.com", "pass")

	if err != nil {
		t.Errorf("Authenticate failed: %v", err)
	}

	if !called {
		t.Error("Login func was not called")
	}
}
