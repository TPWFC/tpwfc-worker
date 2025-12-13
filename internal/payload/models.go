package payload

// Source represents a source reference.
type Source struct {
	SourceID *string `json:"sourceId,omitempty"`
	Name     *string `json:"name,omitempty"`
	Title    *string `json:"title,omitempty"`
	URL      *string `json:"url,omitempty"`
	ID       *string `json:"id,omitempty"`
}

// Note represents a note.
type Note struct {
	Content *string `json:"content,omitempty"`
	ID      *string `json:"id,omitempty"`
}

// FireDuration represents the duration of a fire.
type FireDuration struct {
	Days    *int    `json:"days,omitempty"`
	Hours   *int    `json:"hours,omitempty"`
	Minutes *int    `json:"minutes,omitempty"`
	Seconds *int    `json:"seconds,omitempty"`
	Raw     *string `json:"raw,omitempty"`
}

// KeyStatistics represents key statistics of a fire incident.
type KeyStatistics struct {
	FireDuration          *FireDuration `json:"fireDuration,omitempty"`
	FireLevels            *string       `json:"fireLevels,omitempty"`
	FinalDeaths           *string       `json:"finalDeaths,omitempty"`
	FirefighterCasualties *string       `json:"firefighterCasualties,omitempty"`
	FirefightersDeployed  *int          `json:"firefightersDeployed,omitempty"`
	FireVehicles          *int          `json:"fireVehicles,omitempty"`
	HelpCases             *int          `json:"helpCases,omitempty"`
	HelpCasesProcessed    *int          `json:"helpCasesProcessed,omitempty"`
	AffectedBuildings     *int          `json:"affectedBuildings,omitempty"`
	ShelterUsers          *int          `json:"shelterUsers,omitempty"`
	MissingPersons        *int          `json:"missingPersons,omitempty"`
	UnidentifiedBodies    *int          `json:"unidentifiedBodies,omitempty"`
}

// FireIncident represents the FireIncident collection.
type FireIncident struct {
	SourceHash    *string        `json:"sourceHash,omitempty"`
	Severity      *string        `json:"severity,omitempty"`
	Duration      *FireDuration  `json:"duration,omitempty"`
	IncidentID    *string        `json:"incidentId,omitempty"`
	KeyStatistics *KeyStatistics `json:"keyStatistics,omitempty"`
	Location      *string        `json:"location,omitempty"`
	Map           *string        `json:"map,omitempty"`
	DisasterLevel *string        `json:"disasterLevel,omitempty"`
	FireCause     *string        `json:"fireCause,omitempty"`
	EndDate       string         `json:"endDate"`
	FireName      string         `json:"fireName"`
	FireID        string         `json:"fireId"`
	StartDate     string         `json:"startDate"`
	Sources       []Source       `json:"sources,omitempty"`
	Notes         []Note         `json:"notes,omitempty"`
	ID            int            `json:"id,omitempty"`
	TotalEvents   int            `json:"totalEvents"`
	TotalDeaths   int            `json:"totalDeaths"`
	TotalInjured  int            `json:"totalInjured"`
	TotalMissing  int            `json:"totalMissing"`
}

// Casualties represents the casualties in a fire event.
type Casualties struct {
	Status  *string `json:"status,omitempty"`
	Raw     *string `json:"raw,omitempty"`
	Deaths  int     `json:"deaths"`
	Injured int     `json:"injured"`
	Missing int     `json:"missing"`
}

// Photo represents a photo in a fire event.
type Photo struct {
	Caption *string `json:"caption,omitempty"`
	ID      *string `json:"id,omitempty"`
	URL     string  `json:"url"`
}

// FireEvent represents the FireEvent collection.
type FireEvent struct {
	VideoURL     *string    `json:"videoUrl,omitempty"`
	EventID      string     `json:"eventId"`
	Date         string     `json:"date"`
	Time         string     `json:"time"`
	DateTime     string     `json:"dateTime"`
	Description  string     `json:"description"`
	Category     string     `json:"category"`
	Sources      []Source   `json:"sources,omitempty"`
	Photos       []Photo    `json:"photos,omitempty"`
	Casualties   Casualties `json:"casualties"`
	ID           int        `json:"id,omitempty"`
	FireIncident int        `json:"fireIncident"`
}

// DetailedTimelinePhase represents the DetailedTimelinePhase collection.
type DetailedTimelinePhase struct {
	DateRange     *string `json:"dateRange,omitempty"`
	StartDate     *string `json:"startDate,omitempty"`
	EndDate       *string `json:"endDate,omitempty"`
	Status        *string `json:"status,omitempty"`
	Description   *string `json:"description,omitempty"`
	PhaseID       string  `json:"phaseId"`
	PhaseName     string  `json:"phaseName"`
	PhaseCategory string  `json:"phaseCategory"`
	ID            int     `json:"id,omitempty"`
	FireIncident  int     `json:"fireIncident"`
}

// DetailedTimelineEvent represents the DetailedTimelineEvent collection.
type DetailedTimelineEvent struct {
	StatusNote    *string  `json:"statusNote,omitempty"`
	VideoURL      *string  `json:"videoUrl,omitempty"`
	PhotoURL      *string  `json:"photoUrl,omitempty"`
	EventID       string   `json:"eventId"`
	Date          string   `json:"date"`
	Time          string   `json:"time"`
	DateTime      string   `json:"dateTime"`
	Event         string   `json:"event"`
	Category      string   `json:"category"`
	Sources       []Source `json:"sources,omitempty"`
	ID            int      `json:"id,omitempty"`
	Phase         int      `json:"phase"`
	IsCategoryEnd *bool    `json:"isCategoryEnd,omitempty"`
}

// LongTermTracking represents the LongTermTracking collection.
type LongTermTracking struct {
	Note         *string `json:"note,omitempty"`
	TrackingID   string  `json:"trackingId"`
	Date         string  `json:"date"`
	Category     string  `json:"category"`
	Event        string  `json:"event"`
	Status       string  `json:"status"`
	ID           int     `json:"id,omitempty"`
	FireIncident int     `json:"fireIncident"`
}
