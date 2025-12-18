// Package parsers provides detailed_timeline.md (DETAILED_TIMELINE) parsing functionality.
package parsers

import (
	"fmt"
	"regexp"
	"strings"

	"tpwfc/internal/models"
	"tpwfc/pkg/metadata"
)

// ParseDetailedTimeline parses the detailed timeline markdown and returns a DetailedTimelineDocument.
func (p *Parser) ParseDetailedTimeline(markdown string) (*models.DetailedTimelineDocument, error) {
	// Strip metadata block if present
	meta, cleanMarkdown := metadata.Extract(markdown)
	markdown = cleanMarkdown

	doc := &models.DetailedTimelineDocument{
		Metadata: meta,
	}

	// Parse phases
	doc.Phases = p.parsePhases(markdown)

	// Parse long-term tracking
	doc.LongTermTracking = p.parseLongTermTracking(markdown)

	// Parse category metrics
	doc.CategoryMetrics = p.parseCategoryMetrics(markdown)

	// Parse notes
	doc.Notes = p.parseNotes(markdown)

	return doc, nil
}

// parseCategoryMetrics extracts category metrics from the CATEGORY_METRICS section.
func (p *Parser) parseCategoryMetrics(markdown string) []models.CategoryMetric {
	var metrics []models.CategoryMetric

	lines := strings.Split(markdown, "\n")

	startPattern := regexp.MustCompile(`<!--\s*CATEGORY_METRICS_START\s*-->`)
	endPattern := regexp.MustCompile(`<!--\s*CATEGORY_METRICS_END\s*-->`)

	inSection := false

	for _, line := range lines {
		if startPattern.MatchString(line) {
			inSection = true

			continue
		}

		if endPattern.MatchString(line) {
			break
		}

		if inSection && strings.HasPrefix(line, "|") {
			// Skip header and separator rows
			if strings.Contains(line, "CATEGORY") || strings.Contains(line, "METRIC_KEY") || strings.HasPrefix(line, "|---") || strings.Contains(line, "---") {
				continue
			}

			cells := strings.Split(line, "|")
			// Expected columns: Empty, Category, MetricKey, MetricLabel, MetricValue, MetricUnit, Empty
			if len(cells) < 6 {
				continue
			}

			category := strings.TrimSpace(cells[1])
			metricKey := strings.TrimSpace(cells[2])
			metricLabel := strings.TrimSpace(cells[3])
			metricValueStr := strings.TrimSpace(cells[4])
			metricUnit := strings.TrimSpace(cells[5])

			// Skip invalid rows (empty or separator-like content)
			if category == "" || metricKey == "" || strings.HasPrefix(category, "-") {
				continue
			}

			// Parse metric value as float64
			var metricValue float64
			_, _ = fmt.Sscanf(metricValueStr, "%f", &metricValue)

			metric := models.CategoryMetric{
				Category:    category,
				MetricKey:   metricKey,
				MetricLabel: metricLabel,
				MetricValue: metricValue,
				MetricUnit:  metricUnit,
			}
			metrics = append(metrics, metric)
		}
	}

	return metrics
}

// parsePhases extracts all phases from the detailed timeline markdown.
func (p *Parser) parsePhases(markdown string) []models.DetailedTimelinePhase {
	var phases []models.DetailedTimelinePhase

	lines := strings.Split(markdown, "\n")

	phaseStartPattern := regexp.MustCompile(`<!--\s*PHASE_START\s*-->`)
	phaseEndPattern := regexp.MustCompile(`<!--\s*PHASE_END\s*-->`)
	phaseInfoStartPattern := regexp.MustCompile(`<!--\s*PHASE_INFO_START\s*-->`)
	phaseInfoEndPattern := regexp.MustCompile(`<!--\s*PHASE_INFO_END\s*-->`)
	phaseDescStartPattern := regexp.MustCompile(`<!--\s*PHASE_DESCRIPTION_START\s*-->`)
	phaseDescEndPattern := regexp.MustCompile(`<!--\s*PHASE_DESCRIPTION_END\s*-->`)

	var phaseLines []string

	inPhase := false
	phaseCount := 0

	for _, line := range lines {
		if phaseStartPattern.MatchString(line) {
			inPhase = true
			phaseLines = []string{}

			continue
		}

		if phaseEndPattern.MatchString(line) && inPhase {
			inPhase = false
			phaseCount++

			// Parse the collected phase
			phaseContent := strings.Join(phaseLines, "\n")
			phase := p.parseSinglePhase(phaseContent, phaseCount, phaseInfoStartPattern, phaseInfoEndPattern, phaseDescStartPattern, phaseDescEndPattern)
			phases = append(phases, phase)

			continue
		}

		if inPhase {
			phaseLines = append(phaseLines, line)
		}
	}

	return phases
}

// parseSinglePhase parses a single phase block.
func (p *Parser) parseSinglePhase(content string, phaseNum int, infoStart, infoEnd, descStart, descEnd *regexp.Regexp) models.DetailedTimelinePhase {
	phase := models.DetailedTimelinePhase{
		ID: fmt.Sprintf("phase-%d", phaseNum),
	}

	lines := strings.Split(content, "\n")

	// Parse phase info table
	inInfo := false
	inDesc := false

	var descLines []string

	for _, line := range lines {
		if infoStart.MatchString(line) {
			inInfo = true

			continue
		}

		if infoEnd.MatchString(line) {
			inInfo = false

			continue
		}

		if descStart.MatchString(line) {
			inDesc = true

			continue
		}

		if descEnd.MatchString(line) {
			inDesc = false
			phase.Description = strings.TrimSpace(strings.Join(descLines, " "))

			continue
		}

		if inInfo && strings.HasPrefix(line, "|") && !strings.Contains(line, "KEY") && !strings.HasPrefix(line, "|---") {
			cells := strings.Split(line, "|")
			if len(cells) >= 3 {
				key := strings.TrimSpace(cells[1])
				value := strings.TrimSpace(cells[2])

				switch key {
				case "PHASE_NAME":
					phase.PhaseName = value
				case "PHASE_CATEGORY":
					phase.PhaseCategory = value
				case "DATE_RANGE":
					normalized, start, end := p.parseDateRange(value)
					phase.DateRange = normalized
					phase.StartDate = start
					phase.EndDate = end
				case "STATUS":
					phase.Status = value
				}
			}
		}

		if inDesc {
			trimmed := strings.TrimSpace(line)
			if trimmed != "" {
				descLines = append(descLines, trimmed)
			}
		}
	}

	// Parse events within this phase
	phase.Events = p.parseDetailedTimelineEvents(content)

	return phase
}

// parseDetailedTimelineEvents extracts events from a phase's timeline table.
func (p *Parser) parseDetailedTimelineEvents(phaseContent string) []models.DetailedTimelineEvent {
	var events []models.DetailedTimelineEvent

	lines := strings.Split(phaseContent, "\n")

	inTable := false
	eventCount := 0

	for _, line := range lines {
		if p.tableStartPattern.MatchString(line) {
			inTable = true

			continue
		}

		if p.tableEndPattern.MatchString(line) {
			inTable = false

			continue
		}

		if inTable && strings.HasPrefix(line, "|") {
			// Skip header and separator rows
			if strings.Contains(line, "DATE") || strings.Contains(line, "TIME") || strings.HasPrefix(line, "|---") {
				continue
			}

			cells := strings.Split(line, "|")
			if len(cells) < 7 {
				continue
			}

			dateStr := strings.TrimSpace(cells[1])
			timeStr := strings.TrimSpace(cells[2])
			eventDesc := strings.TrimSpace(cells[3])
			category := strings.TrimSpace(cells[4])
			statusNote := strings.TrimSpace(cells[5])
			sourcesStr := strings.TrimSpace(cells[6])

			// Skip invalid rows
			if dateStr == "" || !regexp.MustCompile(`\d{4}-\d{2}-\d{2}`).MatchString(dateStr) {
				continue
			}

			eventCount++

			// Parse optional video and photo columns
			var videoURL, photoURL string
			if len(cells) > 7 {
				videoURL = parseVideoURL(strings.TrimSpace(cells[7]))
			}

			if len(cells) > 8 {
				photoURL = parseVideoURL(strings.TrimSpace(cells[8])) // Reuse same link extractor
			}

			// Extract end flag from cell 9 (if present)
			var isCategoryEnd bool
			if len(cells) > 9 {
				endStr := strings.TrimSpace(cells[9])
				if strings.EqualFold(endStr, "x") || strings.EqualFold(endStr, "true") {
					isCategoryEnd = true
				}
			}

			// Parse sources for URL extraction
			sourcesRaw := p.parseSources(sourcesStr)
			var sources []models.EventSource
			for _, s := range sourcesRaw {
				sources = append(sources, models.EventSource{
					Name: s.Name,
					URL:  s.URL,
				})
			}

			// Construct DateTime
			var dateTime string
			if timeStr == "TIME_ALL_DAY" || timeStr == "TIME_ONGOING" {
				dateTime = fmt.Sprintf("%sT00:00:00", dateStr)
			} else {
				dateTime = fmt.Sprintf("%sT%s:00", dateStr, normalizeTime(timeStr))
			}

			// Generate event ID using SHA-256 hash (only locale-independent fields)
			eventID := generateEventID(
				dateStr,
				normalizeTime(timeStr),
				category,
			)

			event := models.DetailedTimelineEvent{
				ID:            eventID,
				Date:          dateStr,
				Time:          timeStr,
				DateTime:      dateTime,
				Event:         eventDesc,
				Category:      category,
				StatusNote:    statusNote,
				Sources:       sources,
				VideoURL:      videoURL,
				PhotoURL:      photoURL,
				IsCategoryEnd: isCategoryEnd,
			}
			events = append(events, event)
		}
	}

	return events
}

// parseLongTermTracking extracts long-term tracking events.
func (p *Parser) parseLongTermTracking(markdown string) []models.LongTermTrackingEvent {
	var events []models.LongTermTrackingEvent

	lines := strings.Split(markdown, "\n")

	startPattern := regexp.MustCompile(`<!--\s*LONG_TERM_TRACKING_START\s*-->`)
	endPattern := regexp.MustCompile(`<!--\s*LONG_TERM_TRACKING_END\s*-->`)

	inSection := false
	eventCount := 0

	for _, line := range lines {
		if startPattern.MatchString(line) {
			inSection = true

			continue
		}

		if endPattern.MatchString(line) {
			break
		}

		if inSection && strings.HasPrefix(line, "|") {
			// Skip header and separator rows
			if strings.Contains(line, "DATE") || strings.Contains(line, "CATEGORY") || strings.HasPrefix(line, "|---") {
				continue
			}

			cells := strings.Split(line, "|")
			if len(cells) < 6 {
				continue
			}

			dateStr := strings.TrimSpace(cells[1])
			category := strings.TrimSpace(cells[2])
			eventDesc := strings.TrimSpace(cells[3])
			status := strings.TrimSpace(cells[4])
			note := strings.TrimSpace(cells[5])

			// Skip invalid rows
			if dateStr == "" || !regexp.MustCompile(`\d{4}-\d{2}-\d{2}`).MatchString(dateStr) {
				continue
			}

			eventCount++

			// Generate event ID using SHA-256 hash for long-term tracking
			eventID := generateEventID(
				dateStr,
				"", // No time for long-term tracking
				category,
			)

			event := models.LongTermTrackingEvent{
				ID:       eventID,
				Date:     dateStr,
				Category: category,
				Event:    eventDesc,
				Status:   status,
				Note:     note,
			}
			events = append(events, event)
		}
	}

	return events
}

// parseDateRange normalizes date range string and extracts start/end dates.
func (p *Parser) parseDateRange(raw string) (string, string, string) {
	// Common separators: "至", "to", "-"
	// We normalize to "YYYY-MM-DD" or "YYYY-MM-DD - YYYY-MM-DD"
	raw = strings.TrimSpace(raw)

	// Check for "至" (Chinese 'to')
	if strings.Contains(raw, "至") {
		parts := strings.Split(raw, "至")
		if len(parts) == 2 {
			start := strings.TrimSpace(parts[0])
			end := strings.TrimSpace(parts[1])

			return fmt.Sprintf("%s - %s", start, end), start, end
		}
	}

	// Check for "to"
	if strings.Contains(raw, " to ") {
		parts := strings.Split(raw, " to ")
		if len(parts) == 2 {
			start := strings.TrimSpace(parts[0])
			end := strings.TrimSpace(parts[1])

			return fmt.Sprintf("%s - %s", start, end), start, end
		}
	}

	// Single date or already formatted
	// If it looks like a single date YYYY-MM-DD
	if len(raw) == 10 && strings.Count(raw, "-") == 2 {
		return raw, raw, raw // Start and end are the same
	}

	return raw, raw, raw // Fallback
}
