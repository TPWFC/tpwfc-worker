// Package crawler provides timeline.md (FIRE_TIMELINE) parsing functionality.
package parsers

import (
	"fmt"
	"regexp"
	"strings"

	"tpwfc/internal/models"
	"tpwfc/pkg/metadata"
)

// ParseDuration parses a duration string in dd:hh:mm:ss format.
func ParseDuration(durationStr string) (models.Duration, error) {
	duration := models.Duration{Raw: durationStr}

	// Handle old format hh:mm for backward compatibility
	if strings.Count(durationStr, ":") == 1 {
		parts := strings.Split(durationStr, ":")
		if len(parts) == 2 {
			_, _ = fmt.Sscanf(parts[0], "%d", &duration.Hours)
			_, _ = fmt.Sscanf(parts[1], "%d", &duration.Minutes)
		}

		return duration, nil
	}

	// New format dd:hh:mm:ss
	parts := strings.Split(durationStr, ":")
	if len(parts) != 4 {
		return duration, fmt.Errorf("%w: %s, expected dd:hh:mm:ss", ErrInvalidDurationFormat, durationStr)
	}

	_, _ = fmt.Sscanf(parts[0], "%d", &duration.Days)
	_, _ = fmt.Sscanf(parts[1], "%d", &duration.Hours)
	_, _ = fmt.Sscanf(parts[2], "%d", &duration.Minutes)
	_, _ = fmt.Sscanf(parts[3], "%d", &duration.Seconds)

	return duration, nil
}

// ParseDocument parses the entire markdown document and returns a TimelineDocument.
func (p *Parser) ParseDocument(markdown string) (*models.TimelineDocument, error) {
	// Strip metadata block if present
	meta, cleanMarkdown := metadata.Extract(markdown)
	markdown = cleanMarkdown

	doc := &models.TimelineDocument{
		Metadata: meta,
	}

	// Parse basic info
	doc.BasicInfo = p.parseBasicInfo(markdown)

	// Parse fire cause
	doc.FireCause = p.parseSection(markdown, p.fireCauseStartPattern, p.fireCauseEndPattern)

	// Parse severity
	doc.Severity = p.parseSection(markdown, p.severityStartPattern, p.severityEndPattern)

	// Parse timeline events
	events, err := p.ParseMarkdownTable(markdown)
	if err != nil {
		return nil, err
	}

	doc.Events = events

	// Parse key statistics
	doc.KeyStatistics = p.parseKeyStatistics(markdown)

	// Parse sources
	doc.Sources = p.parseSourcesSection(markdown)

	// Parse notes
	doc.Notes = p.parseNotes(markdown)

	return doc, nil
}

// parseBasicInfo extracts basic information from the BASIC_INFO section.
func (p *Parser) parseBasicInfo(markdown string) models.BasicInfo {
	info := models.BasicInfo{}
	lines := strings.Split(markdown, "\n")
	inSection := false

	for _, line := range lines {
		if p.basicInfoStartPattern.MatchString(line) {
			inSection = true

			continue
		}

		if inSection && strings.HasPrefix(line, "|") && !strings.Contains(line, "項目") && !strings.Contains(line, "KEY") && !strings.HasPrefix(line, "|---") {
			cells := strings.Split(line, "|")
			if len(cells) >= 3 {
				key := strings.TrimSpace(cells[1])
				value := strings.TrimSpace(cells[2])

				switch key {
				case "INCIDENT_ID":
					info.IncidentID = value
				case "INCIDENT_NAME":
					info.IncidentName = value
				case "DATE_RANGE":
					info.DateRange = value
					// Parse start and end dates
					if strings.Contains(value, " - ") {
						parts := strings.Split(value, " - ")
						if len(parts) == 2 {
							info.StartDate = strings.TrimSpace(parts[0])
							info.EndDate = strings.TrimSpace(parts[1])
						}
					} else if strings.Contains(value, "/") {
						parts := strings.Split(value, "/")
						if len(parts) == 2 {
							info.StartDate = strings.TrimSpace(parts[0])
							info.EndDate = strings.TrimSpace(parts[1])
						}
					}
				case "LOCATION":
					info.Location = value
				case "MAP":
					info.Map = value // Keep full markdown [text](url) for translation support
				case "DISASTER_LEVEL":
					info.DisasterLevel = value
				case "DURATION":
					if parsedDuration, err := ParseDuration(value); err == nil {
						info.Duration = parsedDuration
					} else {
						// Fallback to raw string if parsing fails
						info.Duration = models.Duration{Raw: value}
					}
				case AffectedBuildings:
					_, _ = fmt.Sscanf(value, "%d", &info.AffectedBuildings)
				case "SOURCES":
					info.Sources = value
				}
			}
		}
	}

	return info
}

// parseKeyStatistics extracts key statistics from the KEY_STATISTICS section.
func (p *Parser) parseKeyStatistics(markdown string) models.KeyStatistics {
	stats := models.KeyStatistics{}
	lines := strings.Split(markdown, "\n")
	inSection := false

	for _, line := range lines {
		if p.keyStatsStartPattern.MatchString(line) {
			inSection = true

			continue
		}

		if p.keyStatsEndPattern.MatchString(line) {
			break
		}

		if inSection && strings.HasPrefix(line, "|") && !strings.Contains(line, "項目") && !strings.Contains(line, "KEY") && !strings.HasPrefix(line, "|---") {
			cells := strings.Split(line, "|")
			if len(cells) >= 3 {
				key := strings.TrimSpace(cells[1])
				value := strings.TrimSpace(cells[2])

				switch key {
				case "FINAL_DEATHS":
					_, _ = fmt.Sscanf(value, "%d", &stats.FinalDeaths)
				case "FIREFIGHTER_CASUALTIES":
					// Parse "INJURED:x,DEAD:x" format
					stats.FirefighterCasualties = parseFirefighterCasualties(value)
				case "FIREFIGHTERS_DEPLOYED":
					_, _ = fmt.Sscanf(value, "%d", &stats.FirefightersDeployed)
				case "FIRE_VEHICLES":
					_, _ = fmt.Sscanf(value, "%d", &stats.FireVehicles)
				case "HELP_CASES":
					_, _ = fmt.Sscanf(value, "%d", &stats.HelpCases)
				case "HELP_CASES_PROCESSED":
					_, _ = fmt.Sscanf(value, "%d", &stats.HelpCasesProcessed)
				case "SHELTER_USERS":
					_, _ = fmt.Sscanf(value, "%d", &stats.ShelterUsers)
				case "MISSING_PERSONS":
					_, _ = fmt.Sscanf(value, "%d", &stats.MissingPersons)
				case "UNIDENTIFIED_BODIES":
					_, _ = fmt.Sscanf(value, "%d", &stats.UnidentifiedBodies)
				}
			}
		}
	}

	return stats
}

// parseFirefighterCasualties parses the "INJURED:x,DEAD:x" format to FirefighterCasualties struct.
func parseFirefighterCasualties(value string) models.FirefighterCasualties {
	casualties := models.FirefighterCasualties{}

	if value == "" {
		return casualties
	}

	// Parse "INJURED:12,DEAD:1" format
	parts := strings.Split(value, ",")
	for _, part := range parts {
		part = strings.TrimSpace(part)
		if strings.HasPrefix(part, "DEAD:") {
			_, _ = fmt.Sscanf(part, "DEAD:%d", &casualties.Deaths)
		}
		if strings.HasPrefix(part, "INJURED:") {
			_, _ = fmt.Sscanf(part, "INJURED:%d", &casualties.Injured)
		}
	}

	return casualties
}

// parseSourcesSection extracts sources from the SOURCES section.
func (p *Parser) parseSourcesSection(markdown string) []models.Source {
	var sources []models.Source

	lines := strings.Split(markdown, "\n")
	inSection := false

	for _, line := range lines {
		if p.sourcesStartPattern.MatchString(line) {
			inSection = true

			continue
		}

		if p.sourcesEndPattern.MatchString(line) {
			break
		}

		// Skip header row (contains SOURCE_NAME) and separator row (----)
		if inSection && strings.HasPrefix(line, "|") && !strings.Contains(line, "SOURCE_NAME") && !strings.HasPrefix(line, "|---") {
			cells := strings.Split(line, "|")
			// Table format: | NAME | TITLE | URL |
			// After split: ["", NAME, TITLE, URL, ""]
			if len(cells) >= 4 {
				url := strings.TrimSpace(cells[3])
				// Remove angle brackets if present
				url = strings.TrimPrefix(url, "<")
				url = strings.TrimSuffix(url, ">")

				source := models.Source{
					Name:  strings.TrimSpace(cells[1]),
					Title: strings.TrimSpace(cells[2]),
					URL:   url,
				}
				sources = append(sources, source)
			}
		}
	}

	return sources
}

// ParseMarkdownTable extracts timeline events from markdown table.
func (p *Parser) ParseMarkdownTable(markdown string) ([]models.TimelineEvent, error) {
	var events []models.TimelineEvent

	var currentDate string

	var inTable bool

	// Split by lines
	lines := strings.Split(markdown, "\n")

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		// Check for table start marker
		if p.tableStartPattern.MatchString(line) {
			inTable = true

			continue
		}

		// Check for table end marker
		if p.tableEndPattern.MatchString(line) {
			inTable = false

			continue
		}

		// Skip empty lines and table separators
		if line == "" || strings.HasPrefix(line, "|---") {
			continue
		}

		// If we found table boundaries, only parse between them
		if inTable {
			// Parse table row - skip header by checking if first two data columns are headers (日期, 時間)
			if strings.HasPrefix(line, "|") {
				cells := strings.Split(line, "|")
				if len(cells) >= 3 {
					firstCol := strings.TrimSpace(cells[1])
					secondCol := strings.TrimSpace(cells[2])
					// Skip if this is the header row
					if firstCol == "日期" || secondCol == KeywordTimeZhHK || firstCol == "DATE" || secondCol == KeywordTIME {
						continue
					}
				}

				event, err := p.parseTableRow(line, currentDate)
				if err == nil && event != nil {
					events = append(events, *event)
				}
			}

			continue
		}

		// Legacy parsing mode (when no table markers present)
		// Check for date header (multiple formats)
		dateMatch := p.datePattern.FindStringSubmatch(line)
		if len(dateMatch) > 0 {
			month := dateMatch[1]
			day := dateMatch[2]
			currentDate = fmt.Sprintf("2025-%s-%s", padZero(month), padZero(day))

			continue
		}

		// Check alternative date format (### 11月26日（星期一）)
		dateMatchAlt := p.datePatternAlt.FindStringSubmatch(line)
		if len(dateMatchAlt) > 0 {
			month := dateMatchAlt[1]
			day := dateMatchAlt[2]
			currentDate = fmt.Sprintf("2025-%s-%s", padZero(month), padZero(day))

			continue
		}

		// Parse table row (legacy mode)
		if strings.HasPrefix(line, "|") {
			// Split to check if second column is header (時間)
			cells := strings.Split(line, "|")
			if len(cells) >= 3 {
				secondCol := strings.TrimSpace(cells[2])
				// Skip if this is the header row
				if secondCol == "時間" || secondCol == "TIME" {
					continue
				}
			}

			event, err := p.parseTableRow(line, currentDate)
			if err == nil && event != nil {
				events = append(events, *event)
			}
		}
	}

	return events, nil
}

// parseTableRow parses a single table row.
func (p *Parser) parseTableRow(row string, currentDate string) (*models.TimelineEvent, error) {
	// Split row by pipes
	cells := strings.Split(row, "|")

	// Should have at least 7 cells: empty, date, time, description, category, casualties, sources, empty
	// With video column: empty, date, time, description, category, casualties, sources, video, empty
	// With photo column: empty, date, time, description, category, casualties, sources, video, photos, empty
	// With end column: empty, date, time, description, category, casualties, sources, video, photos, end, empty
	if len(cells) < 7 {
		return nil, ErrInsufficientCells
	}

	// Extract date from cell 1 (if present) or use currentDate
	dateStr := strings.TrimSpace(cells[1])
	if dateStr == "" || dateStr == "日期" || dateStr == "DATE" || strings.Contains(dateStr, "11月") {
		// Skip header row or empty date
		return nil, ErrInvalidRow
	}

	// Check if date is in YYYY-MM-DD format
	datePattern := regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
	if datePattern.MatchString(dateStr) {
		// Use the date from the table
		currentDate = dateStr
	}

	timeStr := strings.TrimSpace(cells[2])
	description := strings.TrimSpace(cells[3])
	category := strings.TrimSpace(cells[4])
	casualtiesStr := strings.TrimSpace(cells[5])

	sourcesStr := ""
	if len(cells) > 6 {
		sourcesStr = strings.TrimSpace(cells[6])
	}

	// Extract video URL from cell 7 (if present) - expects markdown link format [text](url)
	var videoURL string

	if len(cells) > 7 {
		videoStr := strings.TrimSpace(cells[7])
		videoURL = parseVideoURL(videoStr)
	}

	// Extract photos from cell 8 (if present)
	var photosRaw []Photo

	if len(cells) > 8 {
		photosStr := strings.TrimSpace(cells[8])
		photosRaw = parsePhotos(photosStr)
	}

	// Extract end flag from cell 9 (if present)
	var isCategoryEnd bool
	if len(cells) > 9 {
		endStr := strings.TrimSpace(cells[9])
		if strings.EqualFold(endStr, "x") || strings.EqualFold(endStr, "true") {
			isCategoryEnd = true
		}
	}

	// Skip invalid rows
	if timeStr == "" || timeStr == "時間" || timeStr == "TIME" {
		return nil, ErrInvalidRow
	}

	// Parse time
	timeStr = strings.TrimSpace(timeStr)
	if !isValidTime(timeStr) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidTimeFormat, timeStr)
	}

	// Parse casualties
	casualtiesData := p.parseCasualties(casualtiesStr)
	casualties := models.CasualtyData{
		Status:  casualtiesData.Status,
		Raw:     casualtiesData.Raw,
		Deaths:  casualtiesData.Deaths,
		Injured: casualtiesData.Injured,
		Missing: casualtiesData.Missing,
	}

	// Parse sources
	sourcesRaw := p.parseSources(sourcesStr)
	var sources []models.EventSource
	var sourceURLs []string
	for _, s := range sourcesRaw {
		sources = append(sources, models.EventSource{
			Name: s.Name,
			URL:  s.URL,
		})
		if s.URL != "" {
			sourceURLs = append(sourceURLs, s.URL)
		}
	}

	// Convert photos
	var photos []models.Photo
	var photoURLs []string
	for _, ph := range photosRaw {
		photos = append(photos, models.Photo{
			Caption: ph.Caption,
			URL:     ph.URL,
		})
		if ph.URL != "" {
			photoURLs = append(photoURLs, ph.URL)
		}
	}

	// Create event ID using SHA-256 hash (only locale-independent fields)
	eventID := generateEventID(
		currentDate,
		normalizeTime(timeStr),
		category,
	)

	// Construct DateTime
	var dateTime string
	if timeStr == TimeAllDay || timeStr == TimeOngoing {
		dateTime = fmt.Sprintf("%sT00:00:00", currentDate)
	} else {
		dateTime = fmt.Sprintf("%sT%s:00", currentDate, normalizeTime(timeStr))
	}

	// Create event
	event := &models.TimelineEvent{
		ID:            eventID,
		Date:          currentDate,
		Time:          timeStr,
		DateTime:      dateTime,
		Description:   description,
		Casualties:    casualties,
		Sources:       sources,
		Category:      category,
		VideoURL:      videoURL,
		Photos:        photos,
		IsCategoryEnd: isCategoryEnd,
	}

	return event, nil
}
