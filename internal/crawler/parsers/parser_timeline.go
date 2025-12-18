// Package parsers provides timeline.md (FIRE_TIMELINE) parsing functionality.
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
					// Parse formatted [text](url) into struct
					// Expected format: [Map Name](https://maps.google.com...)
					matches := regexp.MustCompile(`\[(.*?)\]\((.*?)\)`).FindStringSubmatch(value)
					if len(matches) == 3 {
						info.Map = models.MapSource{
							Name: matches[1],
							URL:  matches[2],
						}
					} else {
						// Fallback if not formatted correctly, assume entire value is URL?
						// Or just put value in Name if URL is missing?
						// Let's assume URL if it starts with http
						if strings.HasPrefix(value, "http") {
							info.Map = models.MapSource{URL: value}
						} else {
							info.Map = models.MapSource{Name: value}
						}
					}
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

	// Pattern to match table separator rows: lines with only |, -, :, and spaces
	separatorPattern := regexp.MustCompile(`^\|[\s\-:\|]+\|$`)

	for _, line := range lines {
		if p.sourcesStartPattern.MatchString(line) {
			inSection = true

			continue
		}

		if p.sourcesEndPattern.MatchString(line) {
			break
		}

		// Skip header row (contains SOURCE_NAME), separator rows (---- patterns), and empty lines
		trimmedLine := strings.TrimSpace(line)
		if inSection && strings.HasPrefix(trimmedLine, "|") && 
			!strings.Contains(line, "SOURCE_NAME") && 
			!separatorPattern.MatchString(trimmedLine) {
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
	var colMap map[string]int

	// Split by lines
	lines := strings.Split(markdown, "\n")

	for i := 0; i < len(lines); i++ {
		line := strings.TrimSpace(lines[i])

		// Check for table start marker
		if p.tableStartPattern.MatchString(line) {
			inTable = true
			colMap = nil // Reset column map for new table
			continue
		}

		// Check for table end marker
		if p.tableEndPattern.MatchString(line) {
			inTable = false
			continue
		}

		// Skip empty lines and table separators
		if line == "" || strings.HasPrefix(line, "|-") || strings.HasPrefix(line, "| -") || strings.Contains(line, "|---") {
			continue
		}

		// If we found table boundaries, only parse between them
		if inTable {
			if strings.HasPrefix(line, "|") {
				cells := strings.Split(line, "|")
				// Filter empty cells from split
				var cleanCells []string
				cleanCells = append(cleanCells, cells...)
				// Remove first and last empty elements often caused by "| data |" split
				if len(cleanCells) > 0 && strings.TrimSpace(cleanCells[0]) == "" {
					cleanCells = cleanCells[1:]
				}
				if len(cleanCells) > 0 && strings.TrimSpace(cleanCells[len(cleanCells)-1]) == "" {
					cleanCells = cleanCells[:len(cleanCells)-1]
				}

				// Check if this is a header row
				isHeader := false
				for _, cell := range cleanCells {
					h := NormalizeHeader(cell)
					if h == ColDate || h == ColTime || h == ColEvent {
						isHeader = true
						break
					}
				}

				if isHeader {
					colMap = make(map[string]int)
					for idx, cell := range cleanCells {
						colMap[NormalizeHeader(cell)] = idx
					}
					continue
				}

				// Only parse if we have a valid column map
				if colMap != nil {
					event, err := p.parseTableRow(cleanCells, currentDate, colMap)
					if err == nil && event != nil {
						events = append(events, *event)
						// Update current date if the row had a specific date
						if event.Date != "" {
							currentDate = event.Date
						}
					}
				}
			}
			continue
		}

		// Legacy parsing mode (when no table markers present or strictly for date headers)
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
	}

	return events, nil
}

// parseTableRow parses a single table row using the column map.
func (p *Parser) parseTableRow(cells []string, currentDate string, colMap map[string]int) (*models.TimelineEvent, error) {
	// Helper to get cell value safely
	getCell := func(colName string) string {
		if idx, ok := colMap[colName]; ok && idx < len(cells) {
			return strings.TrimSpace(cells[idx])
		}
		return ""
	}

	// Extract basic fields
	dateStr := getCell(ColDate)
	timeStr := getCell(ColTime)
	description := getCell(ColEvent)
	category := getCell(ColCategory)
	casualtiesStr := getCell(ColCasualties)
	sourcesStr := getCell(ColSource)
	videoStr := getCell(ColVideo)
	photosStr := getCell(ColPhoto)
	endStr := getCell(ColEnd)

	// Validate essential fields
	if timeStr == "" {
		// If time is missing, it might be a malformed row or separator
		return nil, ErrInvalidRow
	}

	// Update date if present
	datePattern := regexp.MustCompile(`\d{4}-\d{2}-\d{2}`)
	if dateStr != "" && datePattern.MatchString(dateStr) {
		currentDate = dateStr
	}

	// Parse time
	if !isValidTime(timeStr) {
		return nil, fmt.Errorf("%w: %s", ErrInvalidTimeFormat, timeStr)
	}

	// Parse casualties
	casualtiesData := p.parseCasualties(casualtiesStr)
	var casualtyItems []models.CasualtyItem
	for _, item := range casualtiesData.Items {
		casualtyItems = append(casualtyItems, models.CasualtyItem{
			Type:  item.Type,
			Count: item.Count,
		})
	}

	casualties := models.CasualtyData{
		Status: casualtiesData.Status,
		Raw:    casualtiesData.Raw,
		Items:  casualtyItems,
	}

	// Parse sources
	sourcesRaw := p.parseSources(sourcesStr)
	var sources []models.EventSource
	for _, s := range sourcesRaw {
		sources = append(sources, models.EventSource{
			Name: s.Name,
			URL:  s.URL,
		})
	}

	// Parse video
	videoURL := parseVideoURL(videoStr)

	// Parse photos
	photosRaw := parsePhotos(photosStr)
	var photos []models.Photo
	for _, ph := range photosRaw {
		photos = append(photos, models.Photo{
			Caption: ph.Caption,
			URL:     ph.URL,
		})
	}

	// Check end flag
	var isCategoryEnd bool
	if strings.EqualFold(endStr, "x") || strings.EqualFold(endStr, "true") {
		isCategoryEnd = true
	}

	// Create event ID
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
