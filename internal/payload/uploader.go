package payload

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"sync"

	"tpwfc/internal/logger"
	"tpwfc/internal/models"
)

var (
	// ErrIncidentIDRequired is returned when the incident ID is missing in the basic info.
	ErrIncidentIDRequired = errors.New("basicInfo.incidentId is required")
)

// Language/locale constants.
const (
	LangZhHK   = "zh-hk"
	LangZhCN   = "zh-cn"
	LangEnUS   = "en-us"
	LocaleZhHK = "zh_HK"
	LocaleZhCN = "zh_CN"
	LocaleEn   = "en"

	maxConcurrentUploads = 5
)

// Uploader handles uploading timeline data to Payload CMS.
type Uploader struct {
	client Client
	logger *logger.Logger
}

// NewUploader creates a new uploader instance.
func NewUploader(endpoint, apiKey string, log *logger.Logger) *Uploader {
	return &Uploader{
		client: NewGraphQLClient(endpoint, apiKey, log),
		logger: log,
	}
}

// SetSigningSecret sets the HMAC signing secret for request signatures.
func (u *Uploader) SetSigningSecret(secret string) {
	if gqlClient, ok := u.client.(*GraphQLClient); ok {
		gqlClient.SetSigningSecret(secret)
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
// All incident metadata (fireID, fireName, map info) is read from data.BasicInfo.
func (u *Uploader) Upload(data *models.Timeline, language string) (*UploadResult, error) {
	result := &UploadResult{}

	// Step 1: Create or find fire incident
	incidentID, err := u.createOrFindIncident(data, language)
	if err != nil {
		return nil, fmt.Errorf("failed to create/find incident: %w", err)
	}

	result.IncidentID = incidentID
	u.logger.Info(fmt.Sprintf("Fire incident ready: id=%d, fireId=%s", incidentID, data.BasicInfo.IncidentID))

	// Step 2: Upload events concurrently
	u.logger.Info(fmt.Sprintf("Starting upload of %d events...", len(data.Events)))

	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		sem = make(chan struct{}, maxConcurrentUploads)
	)

	for i, event := range data.Events {
		wg.Add(1)
		go func(evt models.TimelineEvent, index int) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			created, err := u.uploadEvent(evt, incidentID, language)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				u.logger.Error(fmt.Sprintf("Failed to upload event %s: %v", evt.ID, err))
				result.Errors = append(result.Errors, err)
				return
			}

			if created {
				result.EventsCreated++
			} else {
				result.EventsUpdated++
			}

			// Progress logging (rough estimate due to concurrency)
			totalProcessed := result.EventsCreated + result.EventsUpdated + len(result.Errors)
			if totalProcessed%10 == 0 || totalProcessed == len(data.Events) {
				u.logger.Info(fmt.Sprintf("Upload progress: %d/%d", totalProcessed, len(data.Events)))
			}
		}(event, i)
	}

	wg.Wait()
	return result, nil
}

// createOrFindIncident creates a new incident or finds an existing one.
func (u *Uploader) createOrFindIncident(data *models.Timeline, language string) (int, error) {
	fireID := data.BasicInfo.IncidentID
	if fireID == "" {
		return 0, ErrIncidentIDRequired
	}

	existingID, err := u.findEntityID(FindFireIncidentQuery, "fireId", fireID, "FireIncidents")
	if err != nil {
		return 0, fmt.Errorf("failed to find existing incident: %w", err)
	}

	incident := u.mapToFireIncident(data)
	locale := u.mapLocale(language)
	variables := map[string]interface{}{
		"data":   incident,
		"locale": locale,
	}

	if existingID > 0 {
		variables["id"] = existingID
		_, err = u.client.Execute(UpdateFireIncidentMutation, variables)
		if err != nil {
			return 0, fmt.Errorf("failed to update incident: %w", err)
		}
		return existingID, nil
	}

	// Create new incident
	resp, err := u.client.Execute(CreateFireIncidentMutation, variables)
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
	existingID, err := u.findEntityID(FindFireEventQuery, "eventId", event.ID, "FireEvents")
	if err != nil {
		return false, fmt.Errorf("failed to find existing event: %w", err)
	}

	eventStruct := u.mapToFireEvent(event, incidentID)
	locale := u.mapLocale(language)
	variables := map[string]interface{}{
		"data":   eventStruct,
		"locale": locale,
	}

	if existingID > 0 {
		variables["id"] = existingID
		_, err = u.client.Execute(UpdateFireEventMutation, variables)
		return false, err
	}

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
	locale := u.mapLocale(language)

	// Step 1: Upload phases and their events
	// Phases are processed sequentially to ensure order/dependencies, but events within phases can be concurrent
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

		// Upload events for this phase concurrently
		u.uploadPhaseEventsConcurrent(phase.Events, phaseID, locale, result)

		if (i+1)%5 == 0 || i == len(data.Phases)-1 {
			u.logger.Info(fmt.Sprintf("Phase upload progress: %d/%d", i+1, len(data.Phases)))
		}
	}

	// Step 2: Upload long-term tracking events concurrently
	u.uploadTrackingConcurrent(data.LongTermTracking, incidentID, locale, result)

	// Step 3: Upload category metrics
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

func (u *Uploader) uploadPhaseEventsConcurrent(events []models.DetailedTimelineEvent, phaseID int, locale string, result *UploadDetailedTimelineResult) {
	uploadConcurrent(u, events, func(evt models.DetailedTimelineEvent) (bool, error) {
		return u.uploadDetailedTimelineEvent(evt, phaseID, locale)
	}, func(created bool) {
		if created {
			result.EventsCreated++
		} else {
			result.EventsUpdated++
		}
	}, "Failed to upload detailed event", result)
}

func (u *Uploader) uploadTrackingConcurrent(trackingEvents []models.LongTermTrackingEvent, incidentID int, locale string, result *UploadDetailedTimelineResult) {
	uploadConcurrent(u, trackingEvents, func(evt models.LongTermTrackingEvent) (bool, error) {
		return u.uploadLongTermTracking(evt, incidentID, locale)
	}, func(created bool) {
		if created {
			result.TrackingCreated++
		} else {
			result.TrackingUpdated++
		}
	}, "Failed to upload tracking", result)
}

func uploadConcurrent[T any](
	u *Uploader,
	items []T,
	uploadFunc func(T) (bool, error),
	onSuccess func(bool),
	logPrefix string,
	result *UploadDetailedTimelineResult,
) {
	var (
		wg  sync.WaitGroup
		mu  sync.Mutex
		sem = make(chan struct{}, maxConcurrentUploads)
	)

	for _, item := range items {
		wg.Add(1)
		go func(val T) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()

			created, err := uploadFunc(val)

			mu.Lock()
			defer mu.Unlock()

			if err != nil {
				u.logger.Error(fmt.Sprintf("%s: %v", logPrefix, err))
				result.Errors = append(result.Errors, err)
				return
			}
			onSuccess(created)
		}(item)
	}
	wg.Wait()
}

// updateIncidentMetrics updates the fire incident with category metrics.
func (u *Uploader) updateIncidentMetrics(incidentID int, metrics []models.CategoryMetric, locale string) error {
	// First, fetch the existing incident to get the map data
	resp, err := u.client.Execute(GetFireIncidentByIDQuery, map[string]interface{}{
		"id": incidentID,
	})
	if err != nil {
		return fmt.Errorf("failed to fetch incident for map data: %w", err)
	}

	type incidentDataStruct struct {
		FireIncident struct {
			Map struct {
				Name string `json:"name"`
				URL  string `json:"url"`
			} `json:"map"`
		} `json:"FireIncident"`
	}
	incidentData, err := UnmarshalGraphQLData[incidentDataStruct](resp)
	if err != nil {
		return fmt.Errorf("failed to parse incident data: %w", err)
	}

	// Prepare metrics data
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

	// Build update data including the required map field
	updateData := map[string]interface{}{
		"categoryMetrics": metricsData,
		"map": map[string]interface{}{
			"name": incidentData.FireIncident.Map.Name,
			"url":  incidentData.FireIncident.Map.URL,
		},
	}

	variables := map[string]interface{}{
		"id":     incidentID,
		"data":   updateData,
		"locale": locale,
	}

	_, err = u.client.Execute(UpdateFireIncidentMutation, variables)
	return err
}

// uploadPhase uploads a single phase, returns phaseID, created flag, error.
func (u *Uploader) uploadPhase(phase models.DetailedTimelinePhase, incidentID int, locale string) (int, bool, error) {
	existingID, err := u.findEntityID(FindDetailedTimelinePhaseQuery, "phaseId", phase.ID, "DetailedTimelinePhases")
	if err != nil {
		return 0, false, fmt.Errorf("failed to find existing phase: %w", err)
	}

	phaseStruct := u.mapToPhase(phase, incidentID)
	variables := map[string]interface{}{
		"data":   phaseStruct,
		"locale": locale,
	}

	if existingID > 0 {
		variables["id"] = existingID
		_, err = u.client.Execute(UpdateDetailedTimelinePhaseMutation, variables)
		return existingID, false, err
	}

	resp, err := u.client.Execute(CreateDetailedTimelinePhaseMutation, variables)
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

func (u *Uploader) uploadDetailedTimelineEvent(event models.DetailedTimelineEvent, phaseID int, locale string) (bool, error) {
	return u.uploadEntity(
		FindDetailedTimelineEventQuery,
		CreateDetailedTimelineEventMutation,
		UpdateDetailedTimelineEventMutation,
		"eventId",
		"DetailedTimelineEvents",
		event.ID,
		u.mapToDetailedEvent(event, phaseID),
		locale,
	)
}

func (u *Uploader) uploadLongTermTracking(tracking models.LongTermTrackingEvent, incidentID int, locale string) (bool, error) {
	return u.uploadEntity(
		FindLongTermTrackingQuery,
		CreateLongTermTrackingMutation,
		UpdateLongTermTrackingMutation,
		"trackingId",
		"LongTermTrackings",
		tracking.ID,
		u.mapToTracking(tracking, incidentID),
		locale,
	)
}

func (u *Uploader) uploadEntity(
	findQuery, createMutation, updateMutation, idKey, responseKey string,
	entityID string,
	data interface{},
	locale string,
) (bool, error) {
	existingID, err := u.findEntityID(findQuery, idKey, entityID, responseKey)
	if err != nil {
		return false, fmt.Errorf("failed to find existing document: %w", err)
	}

	variables := map[string]interface{}{
		"data":   data,
		"locale": locale,
	}

	if existingID > 0 {
		variables["id"] = existingID
		_, err = u.client.Execute(updateMutation, variables)
		return false, err
	}

	_, err = u.client.Execute(createMutation, variables)
	return true, err
}

// --- Helpers ---

// findEntityID performs a find query and returns the ID of the first document if found.
func (u *Uploader) findEntityID(query, varName, varValue, responseKey string) (int, error) {
	resp, err := u.client.Execute(query, map[string]interface{}{
		varName: varValue,
	})
	if err != nil {
		return 0, err
	}
	if resp == nil {
		return 0, nil
	}

	var wrapper map[string]struct {
		Docs []struct {
			ID int `json:"id"`
		} `json:"docs"`
	}

	if err := json.Unmarshal(resp.Data, &wrapper); err != nil {
		return 0, err
	}

	if docList, ok := wrapper[responseKey]; ok && len(docList.Docs) > 0 {
		return docList.Docs[0].ID, nil
	}

	return 0, nil
}

func (u *Uploader) mapLocale(language string) string {
	switch language {
	case LangZhHK:
		return LocaleZhHK
	case LangZhCN:
		return LocaleZhCN
	case LangEnUS:
		return LocaleEn
	default:
		return language
	}
}

func (u *Uploader) mapToFireIncident(data *models.Timeline) FireIncident {
	fireID := data.BasicInfo.IncidentID
	fireName := data.BasicInfo.IncidentName
	if fireName == "" {
		fireName = fireID
	}

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

	if data.Metadata != nil && data.Metadata.Hash != "" {
		incident.SourceHash = strPtr(data.Metadata.Hash)
	}
	if data.BasicInfo.IncidentID != "" {
		incident.IncidentID = strPtr(data.BasicInfo.IncidentID)
	}
	if data.BasicInfo.Location != "" {
		incident.Location = strPtr(data.BasicInfo.Location)
	}
	if data.BasicInfo.Map.Name != "" || data.BasicInfo.Map.URL != "" {
		incident.Map = &Map{
			Name: strPtr(data.BasicInfo.Map.Name),
			URL:  strPtr(data.BasicInfo.Map.URL),
		}
	}
	if data.BasicInfo.DisasterLevel != "" {
		incident.DisasterLevel = strPtr(data.BasicInfo.DisasterLevel)
	}
	if data.BasicInfo.Duration.Raw != "" {
		incident.Duration = &FireDuration{
			Days:    intPtr(data.BasicInfo.Duration.Days),
			Hours:   intPtr(data.BasicInfo.Duration.Hours),
			Minutes: intPtr(data.BasicInfo.Duration.Minutes),
			Seconds: intPtr(data.BasicInfo.Duration.Seconds),
			Raw:     strPtr(data.BasicInfo.Duration.Raw),
		}
	}
	if data.FireCause != "" {
		incident.FireCause = strPtr(data.FireCause)
	}
	if data.Severity != "" {
		incident.Severity = strPtr(data.Severity)
	}

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

	if len(data.Notes) > 0 {
		notes := make([]Note, len(data.Notes))
		for i, n := range data.Notes {
			val := n
			notes[i] = Note{Content: &val}
		}
		incident.Notes = notes
	}

	return incident
}

func (u *Uploader) mapToFireEvent(event models.TimelineEvent, incidentID int) FireEvent {
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

	if event.VideoURL != "" {
		eventStruct.VideoURL = strPtr(event.VideoURL)
	}

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

	return eventStruct
}

func (u *Uploader) mapToPhase(phase models.DetailedTimelinePhase, incidentID int) DetailedTimelinePhase {
	return DetailedTimelinePhase{
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
}

func (u *Uploader) mapToDetailedEvent(event models.DetailedTimelineEvent, phaseID int) DetailedTimelineEvent {
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

	if event.VideoURL != "" {
		eventStruct.VideoURL = strPtr(event.VideoURL)
	}
	if event.PhotoURL != "" {
		eventStruct.PhotoURL = strPtr(event.PhotoURL)
	}

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

	return eventStruct
}

func (u *Uploader) mapToTracking(tracking models.LongTermTrackingEvent, incidentID int) LongTermTracking {
	return LongTermTracking{
		TrackingID:   tracking.ID,
		FireIncident: incidentID,
		Date:         tracking.Date,
		Category:     tracking.Category,
		Event:        tracking.Event,
		Status:       tracking.Status,
		Note:         strPtr(tracking.Note),
	}
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
