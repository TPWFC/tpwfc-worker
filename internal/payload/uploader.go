package payload

import (
	"encoding/json"
	"fmt"
	"os"

	"tpwfc/internal/logger"
	"tpwfc/internal/models"
)

// Language/locale constants.
const (
	LangZhHK   = "zh-hk"
	LangZhCN   = "zh-cn"
	LangEnUS   = "en-us"
	LocaleZhHK = "zh_HK"
	LocaleZhCN = "zh_CN"
	LocaleEn   = "en"
)

// Uploader handles uploading timeline data to Payload CMS.
type Uploader struct {
	client Client
	logger *logger.Logger
}

// NewUploader creates a new uploader instance.
func NewUploader(endpoint, apiKey string, log *logger.Logger) *Uploader {
	return &Uploader{
		client: NewGraphQLClient(endpoint, apiKey),
		logger: log,
	}
}

// NewUploaderWithClient creates a new uploader with a custom client (useful for testing).
func NewUploaderWithClient(client Client, log *logger.Logger) *Uploader {
	return &Uploader{
		client: client,
		logger: log,
	}
}

// Authenticate logs in with email and password.
func (u *Uploader) Authenticate(email, password string) error {
	return u.client.Login(email, password)
}

// UploadResult contains the results of an upload operation.
type UploadResult struct {
	Errors        []error
	IncidentID    int
	EventsCreated int
	EventsUpdated int
}

// LoadTimelineJSON loads timeline data from a JSON file
// Note: This expects the legacy JSON format matching TimelineData.
// Ideally this should be updated to match models.Timeline structure or removed.
func LoadTimelineJSON(filePath string) (*models.Timeline, error) {
	data, err := os.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read file: %w", err)
	}

	var timeline models.Timeline
	if err := json.Unmarshal(data, &timeline); err != nil {
		return nil, fmt.Errorf("failed to parse JSON: %w", err)
	}

	return &timeline, nil
}

// Upload uploads timeline data to Payload CMS.
func (u *Uploader) Upload(data *models.Timeline, fireID, fireName, language string) (*UploadResult, error) {
	result := &UploadResult{}

	// Step 1: Create or find fire incident
	incidentID, err := u.createOrFindIncident(data, fireID, fireName, language)
	if err != nil {
		return nil, fmt.Errorf("failed to create/find incident: %w", err)
	}

	result.IncidentID = incidentID
	u.logger.Info(fmt.Sprintf("Fire incident ready: id=%d, fireId=%s", incidentID, fireID))

	// Step 2: Upload events
	for i, event := range data.Events {
		created, err := u.uploadEvent(event, incidentID, language)
		if err != nil {
			u.logger.Error(fmt.Sprintf("Failed to upload event %s: %v", event.ID, err))
			result.Errors = append(result.Errors, err)

			continue
		}

		if created {
			result.EventsCreated++
		} else {
			result.EventsUpdated++
		}

		// Progress logging every 10 events
		if (i+1)%10 == 0 || i == len(data.Events)-1 {
			u.logger.Info(fmt.Sprintf("Upload progress: %d/%d", i+1, len(data.Events)))
		}
	}

	return result, nil
}

// createOrFindIncident creates a new incident or finds an existing one.
func (u *Uploader) createOrFindIncident(data *models.Timeline, fireID, fireName, language string) (int, error) {
	// Try to find existing incident
	resp, err := u.client.Execute(FindFireIncidentQuery, map[string]interface{}{
		"fireId": fireID,
	})

	existingID := 0

	if err == nil && resp != nil {
		var findResult struct {
			FireIncidents struct {
				Docs []struct {
					ID int `json:"id"`
				} `json:"docs"`
			} `json:"FireIncidents"`
		}

		if unmarshalErr := json.Unmarshal(resp.Data, &findResult); unmarshalErr == nil {
			if len(findResult.FireIncidents.Docs) > 0 {
				existingID = findResult.FireIncidents.Docs[0].ID
			}
		}
	}

	// Prepare incident data using structs
	incident := FireIncident{
		FireID:       fireID,
		FireName:     fireName,
		StartDate:    data.Summary.StartDate,
		EndDate:      data.Summary.EndDate,
		TotalEvents:  data.Summary.TotalEvents,
		TotalDeaths:  data.Summary.TotalDeaths,
		TotalInjured: data.Summary.TotalInjured,
		TotalMissing: data.Summary.TotalMissing,
	}

	// Add source hash from metadata if available
	if data.Metadata != nil && data.Metadata.Hash != "" {
		incident.SourceHash = strPtr(data.Metadata.Hash)
	}

	if data.BasicInfo.IncidentID != "" {
		incident.IncidentID = strPtr(data.BasicInfo.IncidentID)
	}

	if data.BasicInfo.IncidentName != "" {
		incident.FireName = data.BasicInfo.IncidentName
	}

	if data.BasicInfo.Location != "" {
		incident.Location = strPtr(data.BasicInfo.Location)
	}

	if data.BasicInfo.Map.URL != "" || data.BasicInfo.Map.Name != "" {
		incident.Map = &Map{
			Name: strPtr(data.BasicInfo.Map.Name),
			URL:  strPtr(data.BasicInfo.Map.URL),
		}
	}

	if data.BasicInfo.DisasterLevel != "" {
		incident.DisasterLevel = strPtr(data.BasicInfo.DisasterLevel)
	}

	// Duration
	if data.BasicInfo.Duration.Raw != "" {
		incident.Duration = &FireDuration{
			Days:    intPtr(data.BasicInfo.Duration.Days),
			Hours:   intPtr(data.BasicInfo.Duration.Hours),
			Minutes: intPtr(data.BasicInfo.Duration.Minutes),
			Seconds: intPtr(data.BasicInfo.Duration.Seconds),
			Raw:     strPtr(data.BasicInfo.Duration.Raw),
		}
	}

	// Fire Cause and Severity
	if data.FireCause != "" {
		incident.FireCause = strPtr(data.FireCause)
	}

	if data.Severity != "" {
		incident.Severity = strPtr(data.Severity)
	}

	// Key Statistics
	incident.KeyStatistics = &KeyStatistics{
		FinalDeaths: intPtr(data.KeyStatistics.FinalDeaths),
		FirefighterCasualties: &FirefighterCasualties{
			Deaths:  intPtr(data.KeyStatistics.FirefighterCasualties.Deaths),
			Injured: intPtr(data.KeyStatistics.FirefighterCasualties.Injured),
		},
		FirefightersDeployed: intPtr(data.KeyStatistics.FirefightersDeployed),
		FireVehicles:         intPtr(data.KeyStatistics.FireVehicles),
		HelpCases:            intPtr(data.KeyStatistics.HelpCases),
		HelpCasesProcessed:   intPtr(data.KeyStatistics.HelpCasesProcessed),
		ShelterUsers:         intPtr(data.KeyStatistics.ShelterUsers),
		MissingPersons:       intPtr(data.KeyStatistics.MissingPersons),
		UnidentifiedBodies:   intPtr(data.KeyStatistics.UnidentifiedBodies),
	}

	// Sources
	if len(data.Sources) > 0 {
		sources := make([]Source, len(data.Sources))
		for i, s := range data.Sources {
			sources[i] = Source{
				Name:  strPtr(s.Name),
				Title: strPtr(s.Title),
				URL:   strPtr(s.URL),
			}
		}

		incident.Sources = sources
	}

	// Notes
	if len(data.Notes) > 0 {
		notes := make([]Note, len(data.Notes))

		for i, n := range data.Notes {
			// Note: Loop variable capture, but we are creating a new struct each time and using string value
			val := n
			notes[i] = Note{
				Content: &val,
			}
		}

		incident.Notes = notes
	}

	// Map language code to Payload locale enum
	locale := language
	if language == LangZhHK {
		locale = LocaleZhHK
	}

	if language == LangZhCN {
		locale = LocaleZhCN
	}
	if language == LangEnUS {
		locale = LocaleEn
	}

	variables := map[string]interface{}{
		"data":   incident,
		"locale": locale,
	}

	if existingID > 0 {
		variables["id"] = existingID
		// Update existing incident
		_, err = u.client.Execute(UpdateFireIncidentMutation, variables)
		if err != nil {
			return 0, fmt.Errorf("failed to update incident: %w", err)
		}

		return existingID, nil
	}

	// Create new incident
	resp, err = u.client.Execute(CreateFireIncidentMutation, variables)
	if err != nil {
		return 0, err
	}

	var createResult struct {
		CreateFireIncident struct {
			ID int `json:"id"`
		} `json:"createFireIncident"`
	}

	if err := json.Unmarshal(resp.Data, &createResult); err != nil {
		return 0, fmt.Errorf("failed to parse create response: %w", err)
	}

	return createResult.CreateFireIncident.ID, nil
}

// uploadEvent uploads a single event, returns true if created, false if updated.
func (u *Uploader) uploadEvent(event models.TimelineEvent, incidentID int, language string) (bool, error) {
	// Check if event exists
	resp, err := u.client.Execute(FindFireEventQuery, map[string]interface{}{
		"eventId": event.ID,
	})

	existingID := 0

	if err == nil && resp != nil {
		var findResult struct {
			FireEvents struct {
				Docs []struct {
					ID int `json:"id"`
				} `json:"docs"`
			} `json:"FireEvents"`
		}

		if unmarshalErr := json.Unmarshal(resp.Data, &findResult); unmarshalErr == nil {
			if len(findResult.FireEvents.Docs) > 0 {
				existingID = findResult.FireEvents.Docs[0].ID
			}
		}
	}

	// Convert casualty items
	var payloadCasualtyItems []CasualtyItem
	for _, item := range event.Casualties.Items {
		payloadCasualtyItems = append(payloadCasualtyItems, CasualtyItem{
			Type:  item.Type,
			Count: item.Count,
		})
	}

	eventStruct := FireEvent{
		EventID:      event.ID,
		FireIncident: incidentID,
		Date:         event.Date,
		Time:         event.Time,
		DateTime:     event.DateTime,
		Description:  event.Description,
		Category:     event.Category,
		Casualties: Casualties{
			Status: strPtr(event.Casualties.Status),
			Raw:    strPtr(event.Casualties.Raw),
			Items:  payloadCasualtyItems,
		},
	}

	// Add videoUrl if present
	if event.VideoURL != "" {
		eventStruct.VideoURL = strPtr(event.VideoURL)
	}

	// Add sources if present
	if len(event.Sources) > 0 {
		sources := make([]Source, len(event.Sources))
		for i, s := range event.Sources {
			sources[i] = Source{
				Name: strPtr(s.Name),
				URL:  strPtr(s.URL),
			}
		}

		eventStruct.Sources = sources
	}

	// Add photos if present
	if len(event.Photos) > 0 {
		photos := make([]Photo, len(event.Photos))
		for i, p := range event.Photos {
			photos[i] = Photo{
				URL:     p.URL,
				Caption: strPtr(p.Caption),
			}
		}

		eventStruct.Photos = photos
	}

	// Map language code to Payload locale
	locale := language
	if language == LangZhHK {
		locale = LocaleZhHK
	}

	if language == LangZhCN {
		locale = LocaleZhCN
	}
	if language == LangEnUS {
		locale = LocaleEn
	}

	variables := map[string]interface{}{
		"data":   eventStruct,
		"locale": locale,
	}

	if existingID > 0 {
		variables["id"] = existingID
		// Update existing event
		_, err = u.client.Execute(UpdateFireEventMutation, variables)

		return false, err
	}

	// Create new event
	_, err = u.client.Execute(CreateFireEventMutation, variables)

	return true, err
}

// DetailedTimelineData represents the JSON structure for detailed timeline.
type DetailedTimelineData struct {
	Phases           []models.DetailedTimelinePhase `json:"phases"`
	LongTermTracking []models.LongTermTrackingEvent `json:"longTermTracking"`
	CategoryMetrics  []models.CategoryMetric        `json:"categoryMetrics"`
	Notes            []string                       `json:"notes"`
}

// UploadDetailedTimelineResult contains the results of detailed timeline upload.
type UploadDetailedTimelineResult struct {
	Errors          []error
	PhasesCreated   int
	PhasesUpdated   int
	EventsCreated   int
	EventsUpdated   int
	TrackingCreated int
	TrackingUpdated int
	MetricsUpdated  int
}

// UploadDetailedTimeline uploads detailed timeline data to Payload CMS.
func (u *Uploader) UploadDetailedTimeline(data *DetailedTimelineData, incidentID int, language string) (*UploadDetailedTimelineResult, error) {
	result := &UploadDetailedTimelineResult{}

	// Map language code to Payload locale
	locale := language
	if language == LangZhHK {
		locale = LocaleZhHK
	}

	if language == LangZhCN {
		locale = LocaleZhCN
	}
	if language == LangEnUS {
		locale = LocaleEn
	}

	// Upload phases and their events
	for i, phase := range data.Phases {
		phaseID, created, err := u.uploadPhase(phase, incidentID, locale)
		if err != nil {
			u.logger.Error(fmt.Sprintf("Failed to upload phase %s: %v", phase.ID, err))
			result.Errors = append(result.Errors, err)

			continue
		}

		if created {
			result.PhasesCreated++
		} else {
			result.PhasesUpdated++
		}

		// Upload events for this phase
		for _, event := range phase.Events {
			eventCreated, err := u.uploadDetailedTimelineEvent(event, phaseID, locale)
			if err != nil {
				u.logger.Error(fmt.Sprintf("Failed to upload event %s: %v", event.ID, err))
				result.Errors = append(result.Errors, err)

				continue
			}

			if eventCreated {
				result.EventsCreated++
			} else {
				result.EventsUpdated++
			}
		}

		// Progress logging
		if (i+1)%5 == 0 || i == len(data.Phases)-1 {
			u.logger.Info(fmt.Sprintf("Phase upload progress: %d/%d", i+1, len(data.Phases)))
		}
	}

	// Upload long-term tracking events
	for _, tracking := range data.LongTermTracking {
		created, err := u.uploadLongTermTracking(tracking, incidentID, locale)
		if err != nil {
			u.logger.Error(fmt.Sprintf("Failed to upload tracking %s: %v", tracking.ID, err))
			result.Errors = append(result.Errors, err)

			continue
		}

		if created {
			result.TrackingCreated++
		} else {
			result.TrackingUpdated++
		}
	}

	// Upload category metrics to FireIncident
	if len(data.CategoryMetrics) > 0 {
		if err := u.updateIncidentMetrics(incidentID, data.CategoryMetrics, locale); err != nil {
			u.logger.Error(fmt.Sprintf("Failed to upload category metrics: %v", err))
			result.Errors = append(result.Errors, err)
		} else {
			result.MetricsUpdated = len(data.CategoryMetrics)
			u.logger.Info(fmt.Sprintf("Updated %d category metrics", len(data.CategoryMetrics)))
		}
	}

	return result, nil
}

// updateIncidentMetrics updates the fire incident with category metrics.
func (u *Uploader) updateIncidentMetrics(incidentID int, metrics []models.CategoryMetric, locale string) error {
	metricsData := make([]map[string]interface{}, len(metrics))
	for i, m := range metrics {
		metricsData[i] = map[string]interface{}{
			"category":    m.Category,
			"metricKey":   m.MetricKey,
			"metricLabel": m.MetricLabel,
			"metricValue": m.MetricValue,
			"metricUnit":  m.MetricUnit,
		}
	}

	variables := map[string]interface{}{
		"id": incidentID,
		"data": map[string]interface{}{
			"categoryMetrics": metricsData,
		},
		"locale": locale,
	}

	_, err := u.client.Execute(UpdateFireIncidentMutation, variables)
	return err
}

// uploadPhase uploads a single phase, returns phaseID, created flag, error.
func (u *Uploader) uploadPhase(phase models.DetailedTimelinePhase, incidentID int, locale string) (int, bool, error) {
	// Check if phase exists
	resp, err := u.client.Execute(FindDetailedTimelinePhaseQuery, map[string]interface{}{
		"phaseId": phase.ID,
	})

	existingID := 0

	if err == nil && resp != nil {
		var findResult struct {
			DetailedTimelinePhases struct {
				Docs []struct {
					ID int `json:"id"`
				} `json:"docs"`
			} `json:"DetailedTimelinePhases"`
		}

		if unmarshalErr := json.Unmarshal(resp.Data, &findResult); unmarshalErr == nil {
			if len(findResult.DetailedTimelinePhases.Docs) > 0 {
				existingID = findResult.DetailedTimelinePhases.Docs[0].ID
			}
		}
	}

	phaseStruct := DetailedTimelinePhase{
		PhaseID:       phase.ID,
		FireIncident:  incidentID,
		PhaseName:     phase.PhaseName,
		PhaseCategory: phase.PhaseCategory,
		DateRange:     strPtr(phase.DateRange),
		StartDate:     strPtr(phase.StartDate),
		EndDate:       strPtr(phase.EndDate),
		Status:        strPtr(phase.Status),
		Description:   strPtr(phase.Description),
	}

	variables := map[string]interface{}{
		"data":   phaseStruct,
		"locale": locale,
	}

	if existingID > 0 {
		variables["id"] = existingID
		// Update existing phase
		_, err = u.client.Execute(UpdateDetailedTimelinePhaseMutation, variables)

		return existingID, false, err
	}

	// Create new phase
	resp, err = u.client.Execute(CreateDetailedTimelinePhaseMutation, variables)
	if err != nil {
		return 0, false, err
	}

	var createResult struct {
		CreateDetailedTimelinePhase struct {
			ID int `json:"id"`
		} `json:"createDetailedTimelinePhase"`
	}

	if err := json.Unmarshal(resp.Data, &createResult); err != nil {
		return 0, false, fmt.Errorf("failed to parse create response: %w", err)
	}

	return createResult.CreateDetailedTimelinePhase.ID, true, nil
}

// uploadDetailedTimelineEvent uploads a single detailed timeline event.
func (u *Uploader) uploadDetailedTimelineEvent(event models.DetailedTimelineEvent, phaseID int, locale string) (bool, error) {
	// Check if event exists
	resp, err := u.client.Execute(FindDetailedTimelineEventQuery, map[string]interface{}{
		"eventId": event.ID,
	})

	existingID := 0

	if err == nil && resp != nil {
		var findResult struct {
			DetailedTimelineEvents struct {
				Docs []struct {
					ID int `json:"id"`
				} `json:"docs"`
			} `json:"DetailedTimelineEvents"`
		}

		if unmarshalErr := json.Unmarshal(resp.Data, &findResult); unmarshalErr == nil {
			if len(findResult.DetailedTimelineEvents.Docs) > 0 {
				existingID = findResult.DetailedTimelineEvents.Docs[0].ID
			}
		}
	}

	eventStruct := DetailedTimelineEvent{
		EventID:       event.ID,
		Phase:         phaseID,
		Date:          event.Date,
		Time:          event.Time,
		DateTime:      event.DateTime,
		Event:         event.Event,
		Category:      event.Category,
		StatusNote:    strPtr(event.StatusNote),
		IsCategoryEnd: &event.IsCategoryEnd,
	}

	// Add optional fields
	if event.VideoURL != "" {
		eventStruct.VideoURL = strPtr(event.VideoURL)
	}

	if event.PhotoURL != "" {
		eventStruct.PhotoURL = strPtr(event.PhotoURL)
	}

	// Add sources if present
	if len(event.Sources) > 0 {
		sources := make([]Source, len(event.Sources))
		for i, s := range event.Sources {
			sources[i] = Source{
				Name: strPtr(s.Name),
				URL:  strPtr(s.URL),
			}
		}

		eventStruct.Sources = sources
	}

	variables := map[string]interface{}{
		"data":   eventStruct,
		"locale": locale,
	}

	if existingID > 0 {
		variables["id"] = existingID
		// Update existing event
		_, err = u.client.Execute(UpdateDetailedTimelineEventMutation, variables)

		return false, err
	}

	// Create new event
	_, err = u.client.Execute(CreateDetailedTimelineEventMutation, variables)

	return true, err
}

// uploadLongTermTracking uploads a single long-term tracking event.
func (u *Uploader) uploadLongTermTracking(tracking models.LongTermTrackingEvent, incidentID int, locale string) (bool, error) {
	// Check if tracking event exists
	resp, err := u.client.Execute(FindLongTermTrackingQuery, map[string]interface{}{
		"trackingId": tracking.ID,
	})

	existingID := 0

	if err == nil && resp != nil {
		var findResult struct {
			LongTermTrackings struct {
				Docs []struct {
					ID int `json:"id"`
				} `json:"docs"`
			} `json:"LongTermTrackings"`
		}

		if unmarshalErr := json.Unmarshal(resp.Data, &findResult); unmarshalErr == nil {
			if len(findResult.LongTermTrackings.Docs) > 0 {
				existingID = findResult.LongTermTrackings.Docs[0].ID
			}
		}
	}

	trackingStruct := LongTermTracking{
		TrackingID:   tracking.ID,
		FireIncident: incidentID,
		Date:         tracking.Date,
		Category:     tracking.Category,
		Event:        tracking.Event,
		Status:       tracking.Status,
		Note:         strPtr(tracking.Note),
	}

	variables := map[string]interface{}{
		"data":   trackingStruct,
		"locale": locale,
	}

	if existingID > 0 {
		variables["id"] = existingID
		// Update existing tracking
		_, err = u.client.Execute(UpdateLongTermTrackingMutation, variables)

		return false, err
	}

	// Create new tracking
	_, err = u.client.Execute(CreateLongTermTrackingMutation, variables)

	return true, err
}

// Helpers for converting values to pointers

func strPtr(s string) *string {
	if s == "" {
		return nil
	}

	return &s
}

func intPtr(i int) *int {
	return &i
}
