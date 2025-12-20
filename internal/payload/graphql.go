// Package payload provides client functionality for interacting with Payload CMS GraphQL API.
package payload

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"sync"
	"time"

	"tpwfc/internal/logger"
)

// GraphQL errors.
var (
	ErrUnexpectedStatusCode = errors.New("unexpected status code")
	ErrGraphQLError         = errors.New("graphql error")
	ErrNoTokenReceived      = errors.New("no token received from login")
	ErrNoData               = errors.New("no data in response")
)

// Client defines the interface for GraphQL communication.
type Client interface {
	Execute(query string, variables map[string]interface{}) (*GraphQLResponse, error)
	Login(email, password string) error
}

// Ensure GraphQLClient implements Client.
var _ Client = (*GraphQLClient)(nil)

// GraphQLClient handles GraphQL communication with Payload CMS.
type GraphQLClient struct {
	httpClient *http.Client
	endpoint   string
	apiKey     string
	authToken  string
	mu         sync.RWMutex
	logger     *logger.Logger
}

// GraphQLRequest represents a GraphQL request.
type GraphQLRequest struct {
	Variables map[string]interface{} `json:"variables,omitempty"`
	Query     string                 `json:"query"`
}

// GraphQLResponse represents a GraphQL response.
type GraphQLResponse struct {
	Data   json.RawMessage `json:"data"`
	Errors []GraphQLError  `json:"errors,omitempty"`
}

// GraphQLError represents a GraphQL error.
type GraphQLError struct {
	Message   string `json:"message"`
	Locations []struct {
		Line   int `json:"line"`
		Column int `json:"column"`
	} `json:"locations,omitempty"`
	Path []interface{} `json:"path,omitempty"`
}

// NewGraphQLClient creates a new GraphQL client.
func NewGraphQLClient(endpoint, apiKey string, log *logger.Logger) *GraphQLClient {
	return &GraphQLClient{
		endpoint: endpoint,
		apiKey:   apiKey,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
		logger: log,
	}
}

// Execute sends a GraphQL request and returns the response.
func (c *GraphQLClient) Execute(query string, variables map[string]interface{}) (*GraphQLResponse, error) {
	if c.logger != nil {
		c.logger.Debug(fmt.Sprintf("Executing GraphQL query: %s...", query[:min(len(query), 50)]))
	}

	reqBody := GraphQLRequest{
		Query:     query,
		Variables: variables,
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, c.endpoint, bytes.NewBuffer(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")

	c.mu.RLock()
	token := c.authToken
	key := c.apiKey
	c.mu.RUnlock()

	if token != "" {
		// Use authenticated token if available
		req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", token))
	} else if key != "" {
		// Fall back to API key if no auth token
		req.Header.Set("Authorization", key)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer func() {
		if closeErr := resp.Body.Close(); closeErr != nil && err == nil {
			err = fmt.Errorf("failed to close response body: %w", closeErr)
		}
	}()

	// Limit response size to 10MB
	reader := io.LimitReader(resp.Body, 10*1024*1024)

	body, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		if c.logger != nil {
			c.logger.Error(fmt.Sprintf("GraphQL request failed with status %d: %s", resp.StatusCode, string(body)))
		}
		return nil, fmt.Errorf("%w: %d: %s", ErrUnexpectedStatusCode, resp.StatusCode, string(body))
	}

	var gqlResp GraphQLResponse
	if err := json.Unmarshal(body, &gqlResp); err != nil {
		return nil, fmt.Errorf("failed to parse response: %w", err)
	}

	if len(gqlResp.Errors) > 0 {
		return &gqlResp, fmt.Errorf("%w: %s", ErrGraphQLError, gqlResp.Errors[0].Message)
	}

	return &gqlResp, nil
}

// UnmarshalGraphQLData unmarshals the response data into the target struct.
func UnmarshalGraphQLData[T any](resp *GraphQLResponse) (*T, error) {
	if resp == nil || resp.Data == nil {
		return nil, ErrNoData
	}
	var target T
	if err := json.Unmarshal(resp.Data, &target); err != nil {
		return nil, fmt.Errorf("failed to parse response data: %w", err)
	}
	return &target, nil
}

// CreateFireIncidentMutation creates a new fire incident.
const CreateFireIncidentMutation = `
mutation CreateFireIncident($data: mutationFireIncidentInput!, $locale: LocaleInputType) {
  createFireIncident(data: $data, locale: $locale) {
    id
    fireId
    fireName
  }
}
`

// UpdateFireIncidentMutation updates an existing fire incident.
const UpdateFireIncidentMutation = `
mutation UpdateFireIncident($id: Int!, $data: mutationFireIncidentUpdateInput!, $locale: LocaleInputType) {
  updateFireIncident(id: $id, data: $data, locale: $locale) {
    id
    fireId
    fireName
  }
}
`

// CreateFireEventMutation creates a new fire event.
const CreateFireEventMutation = `
mutation CreateFireEvent($data: mutationFireEventInput!, $locale: LocaleInputType) {
  createFireEvent(data: $data, locale: $locale) {
    id
    eventId
  }
}
`

// FindFireIncidentQuery finds a fire incident by fire ID.
const FindFireIncidentQuery = `
query FindFireIncident($fireId: String!) {
  FireIncidents(where: { fireId: { equals: $fireId } }, limit: 1) {
    docs {
      id
      fireId
      fireName
    }
  }
}
`

// GetFireIncidentByIDQuery gets a fire incident by ID including map data.
const GetFireIncidentByIDQuery = `
query GetFireIncidentByID($id: Int!) {
  FireIncident(id: $id) {
    id
    fireId
    fireName
    map {
      name
      url
    }
  }
}
`

// FindFireEventQuery finds a fire event by event ID.
const FindFireEventQuery = `
query FindFireEvent($eventId: String!) {
  FireEvents(where: { eventId: { equals: $eventId } }, limit: 1) {
    docs {
      id
      eventId
    }
  }
}
`

// UpdateFireEventMutation updates an existing fire event.
const UpdateFireEventMutation = `
mutation UpdateFireEvent($id: Int!, $data: mutationFireEventUpdateInput!, $locale: LocaleInputType) {
  updateFireEvent(id: $id, data: $data, locale: $locale) {
    id
    eventId
  }
}
`

// LoginUserMutation authenticates user and returns token.
const LoginUserMutation = `
mutation LoginUser($email: String!, $password: String!) {
  loginUser(email: $email, password: $password) {
    token
    user {
      id
      email
    }
  }
}
`

// Login authenticates with email and password, storing the auth token.
func (c *GraphQLClient) Login(email, password string) error {
	resp, err := c.Execute(LoginUserMutation, map[string]interface{}{
		"email":    email,
		"password": password,
	})

	if err != nil {
		return fmt.Errorf("login failed: %w", err)
	}

	var loginResp struct {
		LoginUser struct {
			Token string `json:"token"`
			User  struct {
				Email string `json:"email"`
				ID    int    `json:"id"`
			} `json:"user"`
		} `json:"loginUser"`
	}

	if err := json.Unmarshal(resp.Data, &loginResp); err != nil {
		return fmt.Errorf("failed to parse login response: %w", err)
	}

	if loginResp.LoginUser.Token == "" {
		return ErrNoTokenReceived
	}

	c.authToken = loginResp.LoginUser.Token

	return nil
}

// Detailed Timeline Phase mutations and queries

// CreateDetailedTimelinePhaseMutation creates a new detailed timeline phase.
const CreateDetailedTimelinePhaseMutation = `
mutation CreateDetailedTimelinePhase($data: mutationDetailedTimelinePhaseInput!, $locale: LocaleInputType) {
  createDetailedTimelinePhase(data: $data, locale: $locale) {
    id
    phaseId
    phaseName
  }
}
`

// UpdateDetailedTimelinePhaseMutation updates an existing detailed timeline phase.
const UpdateDetailedTimelinePhaseMutation = `
mutation UpdateDetailedTimelinePhase($id: Int!, $data: mutationDetailedTimelinePhaseUpdateInput!, $locale: LocaleInputType) {
  updateDetailedTimelinePhase(id: $id, data: $data, locale: $locale) {
    id
    phaseId
    phaseName
  }
}
`

// FindDetailedTimelinePhaseQuery finds a detailed timeline phase by phase ID.
const FindDetailedTimelinePhaseQuery = `
query FindDetailedTimelinePhase($phaseId: String!) {
  DetailedTimelinePhases(where: { phaseId: { equals: $phaseId } }, limit: 1) {
    docs {
      id
      phaseId
      phaseName
    }
  }
}
`

// Detailed Timeline Event mutations and queries

// CreateDetailedTimelineEventMutation creates a new detailed timeline event.
const CreateDetailedTimelineEventMutation = `
mutation CreateDetailedTimelineEvent($data: mutationDetailedTimelineEventInput!, $locale: LocaleInputType) {
  createDetailedTimelineEvent(data: $data, locale: $locale) {
    id
    eventId
  }
}
`

// UpdateDetailedTimelineEventMutation updates an existing detailed timeline event.
const UpdateDetailedTimelineEventMutation = `
mutation UpdateDetailedTimelineEvent($id: Int!, $data: mutationDetailedTimelineEventUpdateInput!, $locale: LocaleInputType) {
  updateDetailedTimelineEvent(id: $id, data: $data, locale: $locale) {
    id
    eventId
  }
}
`

// FindDetailedTimelineEventQuery finds a detailed timeline event by event ID.
const FindDetailedTimelineEventQuery = `
query FindDetailedTimelineEvent($eventId: String!) {
  DetailedTimelineEvents(where: { eventId: { equals: $eventId } }, limit: 1) {
    docs {
      id
      eventId
    }
  }
}
`

// Long Term Tracking mutations and queries

// CreateLongTermTrackingMutation creates a new long-term tracking entry.
const CreateLongTermTrackingMutation = `
mutation CreateLongTermTracking($data: mutationLongTermTrackingInput!, $locale: LocaleInputType) {
  createLongTermTracking(data: $data, locale: $locale) {
    id
    trackingId
  }
}
`

// UpdateLongTermTrackingMutation updates an existing long-term tracking entry.
const UpdateLongTermTrackingMutation = `
mutation UpdateLongTermTracking($id: Int!, $data: mutationLongTermTrackingUpdateInput!, $locale: LocaleInputType) {
  updateLongTermTracking(id: $id, data: $data, locale: $locale) {
    id
    trackingId
  }
}
`

// FindLongTermTrackingQuery finds a long-term tracking entry by tracking ID.
const FindLongTermTrackingQuery = `
query FindLongTermTracking($trackingId: String!) {
  LongTermTrackings(where: { trackingId: { equals: $trackingId } }, limit: 1) {
    docs {
      id
      trackingId
    }
  }
}
`
