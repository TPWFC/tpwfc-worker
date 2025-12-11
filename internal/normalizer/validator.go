package normalizer

import (
	"errors"
	"fmt"

	"tpwfc/internal/models"
)

// Validation errors.
var (
	ErrInvalidDataType      = errors.New("invalid data type: expected *models.TimelineDocument")
	ErrMissingIncidentID    = errors.New("missing incident ID in basic info")
	ErrMissingIncidentName  = errors.New("missing incident name in basic info")
	ErrNoEvents             = errors.New("timeline document contains no events")
	ErrEventMissingDate     = errors.New("event missing date")
	ErrEventMissingTime     = errors.New("event missing time")
	ErrEventMissingDateTime = errors.New("event missing datetime")
	ErrNoSources            = errors.New("timeline document contains no sources")
)

// Validator handles data validation.
type Validator struct {
	// Add validation rules if needed
}

// NewValidator creates a new validator instance.
func NewValidator() *Validator {
	return &Validator{}
}

// Validate checks if data meets requirements.
func (v *Validator) Validate(data interface{}) error {
	doc, ok := data.(*models.TimelineDocument)
	if !ok {
		return ErrInvalidDataType
	}

	if doc.BasicInfo.IncidentID == "" {
		return ErrMissingIncidentID
	}

	if doc.BasicInfo.IncidentName == "" {
		return ErrMissingIncidentName
	}

	if len(doc.Events) == 0 {
		return ErrNoEvents
	}

	// Validate events
	for i, event := range doc.Events {
		if event.Date == "" {
			return fmt.Errorf("%w at index %d", ErrEventMissingDate, i)
		}

		if event.Time == "" {
			return fmt.Errorf("%w at index %d", ErrEventMissingTime, i)
		}
		// DateTime is constructed by parser, so should be present if Date/Time are valid
		if event.DateTime == "" {
			return fmt.Errorf("%w at index %d", ErrEventMissingDateTime, i)
		}
	}

	if len(doc.Sources) == 0 {
		// Just a warning in logs usually, but here strict validation?
		// Let's assume at least one source is required for credibility
		return ErrNoSources
	}

	return nil
}
