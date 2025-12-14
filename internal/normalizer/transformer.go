package normalizer

import (
	"errors"
	"regexp"
	"strconv"
	"time"

	"tpwfc/internal/models"
)

// ErrInvalidTransformerDataType is returned when the data type is invalid.
var ErrInvalidTransformerDataType = errors.New("invalid data type: expected *models.TimelineDocument")

// Transformer handles data format transformations.
type Transformer struct {
	numberPattern *regexp.Regexp
}

// NewTransformer creates a new transformer instance.
func NewTransformer() *Transformer {
	return &Transformer{
		numberPattern: regexp.MustCompile(`(\d+)`),
	}
}

// Transform converts data into target format.
func (t *Transformer) Transform(data interface{}) (interface{}, error) {
	doc, ok := data.(*models.TimelineDocument)
	if !ok {
		return nil, ErrInvalidTransformerDataType
	}

	now := time.Now()

	timeline := &models.Timeline{
		BasicInfo:     doc.BasicInfo,
		FireCause:     doc.FireCause,
		Severity:      doc.Severity,
		Events:        doc.Events,
		KeyStatistics: doc.KeyStatistics,
		Sources:       doc.Sources,
		Notes:         doc.Notes,
		CreatedAt:     now,
		UpdatedAt:     now,
		Metadata:      doc.Metadata,
	}

	// Calculate summary statistics
	totalDeaths := doc.KeyStatistics.FinalDeaths

	// Aggregate injured from events if not available in KeyStatistics
	totalInjured := 0
	for _, event := range doc.Events {
		totalInjured += event.Casualties.Injured
	}

	// KeyStatistics might have help cases which could be used, but event aggregation is likely more accurate for "Injured"
	// unless KeyStatistics has a specific field we missed.

	summary := models.TimelineSummary{
		Title:        doc.BasicInfo.IncidentName,
		StartDate:    doc.BasicInfo.StartDate,
		EndDate:      doc.BasicInfo.EndDate,
		TotalEvents:  len(doc.Events),
		TotalDeaths:  totalDeaths,
		TotalInjured: totalInjured,
		TotalMissing: doc.KeyStatistics.MissingPersons,
		Description:  doc.BasicInfo.DateRange,
	}

	timeline.Summary = summary

	return timeline, nil
}

// parseStatInt extracts the first number found in a string.
func (t *Transformer) parseStatInt(s string) int {
	match := t.numberPattern.FindString(s)
	if match == "" {
		return 0
	}

	val, err := strconv.Atoi(match)
	if err != nil {
		return 0
	}

	return val
}
