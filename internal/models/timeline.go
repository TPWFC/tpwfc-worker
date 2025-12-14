package models

import (
	"time"

	"tpwfc/pkg/metadata"
)

// TimelineDocument represents the complete parsed document.
type TimelineDocument struct {
	Metadata      *metadata.Metadata `json:"metadata"`
	BasicInfo     BasicInfo          `json:"basicInfo"`
	FireCause     string             `json:"fireCause"`
	Severity      string             `json:"severity"`
	Events        []TimelineEvent    `json:"timeline"`
	Sources       []Source           `json:"sources"`
	Notes         []string           `json:"notes"`
	KeyStatistics KeyStatistics      `json:"keyStatistics"`
}

// BasicInfo holds the basic incident information.
type BasicInfo struct {
	IncidentID        string   `json:"incidentId"`
	IncidentName      string   `json:"incidentName"`
	DateRange         string   `json:"dateRange"`
	StartDate         string   `json:"startDate"`
	EndDate           string   `json:"endDate"`
	Location          string   `json:"location"`
	Map               string   `json:"map"`
	DisasterLevel     string   `json:"disasterLevel"`
	Sources           string   `json:"sources"`
	Duration          Duration `json:"duration"`
	AffectedBuildings int      `json:"affectedBuildings"`
}

// Timeline represents a complete timeline of events.
type Timeline struct {
	UpdatedAt     time.Time          `json:"updatedAt"`
	CreatedAt     time.Time          `json:"createdAt"`
	Metadata      *metadata.Metadata `json:"metadata"`
	BasicInfo     BasicInfo          `json:"basicInfo"`
	Summary       TimelineSummary    `json:"summary"`
	Severity      string             `json:"severity"`
	FireCause     string             `json:"fireCause"`
	Events        []TimelineEvent    `json:"timeline"`
	Sources       []Source           `json:"sources"`
	Notes         []string           `json:"notes"`
	KeyStatistics KeyStatistics      `json:"keyStatistics"`
}

// TimelineEvent represents a single event in the timeline.
type TimelineEvent struct {
	ID            string        `json:"id"`
	Date          string        `json:"date"`
	Time          string        `json:"time"`
	DateTime      string        `json:"dateTime"`
	Description   string        `json:"description"`
	Category      string        `json:"category"`
	VideoURL      string        `json:"videoUrl,omitempty"`
	Sources       []EventSource `json:"sources"`
	Photos        []Photo       `json:"photos,omitempty"`
	Casualties    CasualtyData  `json:"casualties"`
	IsCategoryEnd bool          `json:"isCategoryEnd"`
}

// CasualtyData holds casualty statistics.
type CasualtyData struct {
	Status  string `json:"status"`
	Raw     string `json:"raw"`
	Deaths  int    `json:"deaths"`
	Injured int    `json:"injured"`
	Missing int    `json:"missing"`
}

// EventSource represents a reference source attached to an event.
type EventSource struct {
	Name string `json:"name"`
	URL  string `json:"url"`
}

// Photo represents a photo with optional caption.
type Photo struct {
	URL     string `json:"url"`
	Caption string `json:"caption,omitempty"`
}

// Source represents a reference source with Name, Title, and URL (document level).
type Source struct {
	Name  string `json:"name"`
	Title string `json:"title"`
	URL   string `json:"url"`
}

// FirefighterCasualties holds firefighter casualty counts.
type FirefighterCasualties struct {
	Deaths  int `json:"deaths"`
	Injured int `json:"injured"`
}

// KeyStatistics holds key statistics data.
type KeyStatistics struct {
	FinalDeaths           int                   `json:"finalDeaths"`
	FirefighterCasualties FirefighterCasualties `json:"firefighterCasualties"`
	FirefightersDeployed  int                   `json:"firefightersDeployed"`
	FireVehicles          int                   `json:"fireVehicles"`
	HelpCases             int                   `json:"helpCases"`
	HelpCasesProcessed    int                   `json:"helpCasesProcessed"`
	ShelterUsers          int                   `json:"shelterUsers"`
	MissingPersons        int                   `json:"missingPersons"`
	UnidentifiedBodies    int                   `json:"unidentifiedBodies"`
}

// Duration represents a duration.
type Duration struct {
	Raw     string `json:"raw"`
	Days    int    `json:"days"`
	Hours   int    `json:"hours"`
	Minutes int    `json:"minutes"`
	Seconds int    `json:"seconds"`
}

// TimelineSummary holds aggregate statistics.
type TimelineSummary struct {
	Title        string `json:"title"`
	StartDate    string `json:"startDate"`
	EndDate      string `json:"endDate"`
	Description  string `json:"description"`
	TotalEvents  int    `json:"totalEvents"`
	TotalDeaths  int    `json:"totalDeaths"`
	TotalInjured int    `json:"totalInjured"`
	TotalMissing int    `json:"totalMissing"`
}

// DetailedTimelineDocument represents the complete detailed timeline.
type DetailedTimelineDocument struct {
	Metadata         *metadata.Metadata      `json:"metadata"`
	Phases           []DetailedTimelinePhase `json:"phases"`
	LongTermTracking []LongTermTrackingEvent `json:"longTermTracking"`
	CategoryMetrics  []CategoryMetric        `json:"categoryMetrics"`
	Notes            []string                `json:"notes"`
}

// DetailedTimelinePhase represents a phase in the detailed timeline.
type DetailedTimelinePhase struct {
	ID            string                  `json:"id"`
	PhaseName     string                  `json:"phaseName"`
	PhaseCategory string                  `json:"phaseCategory"`
	DateRange     string                  `json:"dateRange"`
	StartDate     string                  `json:"startDate"`
	EndDate       string                  `json:"endDate"`
	Status        string                  `json:"status"`
	Description   string                  `json:"description"`
	Events        []DetailedTimelineEvent `json:"events"`
}

// DetailedTimelineEvent represents an event within a phase.
type DetailedTimelineEvent struct {
	ID            string        `json:"id"`
	Date          string        `json:"date"`
	Time          string        `json:"time"`
	DateTime      string        `json:"dateTime"`
	Event         string        `json:"event"`
	Category      string        `json:"category"`
	StatusNote    string        `json:"statusNote"`
	VideoURL      string        `json:"videoUrl,omitempty"`
	PhotoURL      string        `json:"photoUrl,omitempty"`
	Sources       []EventSource `json:"sources"`
	IsCategoryEnd bool          `json:"isCategoryEnd"`
}

// LongTermTrackingEvent represents a long-term or future event.
type LongTermTrackingEvent struct {
	ID       string `json:"id"`
	Date     string `json:"date"`
	Category string `json:"category"`
	Event    string `json:"event"`
	Status   string `json:"status"`
	Note     string `json:"note"`
}

// CategoryMetric represents a single metric for a category.
type CategoryMetric struct {
	Category    string  `json:"category"`
	MetricKey   string  `json:"metricKey"`
	MetricLabel string  `json:"metricLabel"`
	MetricValue float64 `json:"metricValue"`
	MetricUnit  string  `json:"metricUnit"`
}
