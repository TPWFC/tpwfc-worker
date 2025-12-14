// Package crawler provides web scraping and markdown parsing functionality for extracting fire timeline data.
package crawler

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"tpwfc/internal/models"
	"tpwfc/pkg/metadata"
)

// Common constants.
const (
	StatusNone        = "STATUS_NONE"
	TimeOngoing       = "TIME_ONGOING"
	TimeAllDay        = "TIME_ALL_DAY"
	DateRange         = "DATE_RANGE"
	AffectedBuildings = "AFFECTED_BUILDINGS"
	KeywordTIME       = "TIME"
	KeywordTimeZhHK   = "時間"
)

// Parser errors.
var (
	ErrInvalidDurationFormat = errors.New("invalid duration format")
	ErrInsufficientCells     = errors.New("insufficient cells in row")
	ErrInvalidRow            = errors.New("invalid row")
	ErrInvalidTimeFormat     = errors.New("invalid time format")
)

// Parser handles markdown parsing and data extraction.
type Parser struct {
	datePattern    *regexp.Regexp
	datePatternAlt *regexp.Regexp
	datePatternISO *regexp.Regexp
	linkPattern    *regexp.Regexp
	numberPattern  *regexp.Regexp
	// Section boundary patterns
	basicInfoStartPattern *regexp.Regexp
	basicInfoEndPattern   *regexp.Regexp
	fireCauseStartPattern *regexp.Regexp
	fireCauseEndPattern   *regexp.Regexp
	severityStartPattern  *regexp.Regexp
	severityEndPattern    *regexp.Regexp
	tableStartPattern     *regexp.Regexp
	tableEndPattern       *regexp.Regexp
	keyStatsStartPattern  *regexp.Regexp
	keyStatsEndPattern    *regexp.Regexp
	sourcesStartPattern   *regexp.Regexp
	sourcesEndPattern     *regexp.Regexp
	notesStartPattern     *regexp.Regexp
	notesEndPattern       *regexp.Regexp
}

// NewParser creates a new parser instance.
func NewParser() *Parser {
	return &Parser{
		// Pattern for **11月26日** format
		datePattern: regexp.MustCompile(`\*\*(\d{1,2})月(\d{1,2})日\*\*`),
		// Pattern for ### 11月26日（星期一） format (markdown heading with optional weekday)
		datePatternAlt: regexp.MustCompile(`^#{1,3}\s*(\d{1,2})月(\d{1,2})日`),
		// Pattern for ISO date format YYYY-MM-DD
		datePatternISO: regexp.MustCompile(`^(\d{4})-(\d{1,2})-(\d{1,2})$`),
		linkPattern:    regexp.MustCompile(`\[(.*?)\]\((.*?)\)`),
		numberPattern:  regexp.MustCompile(`(\d+)\s*(死|傷|失蹤|人)`),
		// Section boundary patterns
		basicInfoStartPattern: regexp.MustCompile(`<!--\s*BASIC_INFO_START\s*-->`),
		basicInfoEndPattern:   regexp.MustCompile(`<!--\s*BASIC_INFO_END\s*-->`),
		fireCauseStartPattern: regexp.MustCompile(`<!--\s*FIRE_CAUSE_START\s*-->`),
		fireCauseEndPattern:   regexp.MustCompile(`<!--\s*FIRE_CAUSE_END\s*-->`),
		severityStartPattern:  regexp.MustCompile(`<!--\s*SEVERITY_START\s*-->`),
		severityEndPattern:    regexp.MustCompile(`<!--\s*SEVERITY_END\s*-->`),
		tableStartPattern:     regexp.MustCompile(`<!--\s*TIMELINE_TABLE_START\s*-->`),
		tableEndPattern:       regexp.MustCompile(`<!--\s*TIMELINE_TABLE_END\s*-->`),
		keyStatsStartPattern:  regexp.MustCompile(`<!--\s*KEY_STATISTICS_START\s*-->`),
		keyStatsEndPattern:    regexp.MustCompile(`<!--\s*KEY_STATISTICS_END\s*-->`),
		sourcesStartPattern:   regexp.MustCompile(`<!--\s*SOURCES_START\s*-->`),
		sourcesEndPattern:     regexp.MustCompile(`<!--\s*SOURCES_END\s*-->`),
		notesStartPattern:     regexp.MustCompile(`<!--\s*NOTES_START\s*-->`),
		notesEndPattern:       regexp.MustCompile(`<!--\s*NOTES_END\s*-->`),
	}
}

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

// parseSection extracts text content between start and end markers.
// Filters out HTML comment tags like <!-- TRANSLATE_TEXT -->.
func (p *Parser) parseSection(markdown string, startPattern, endPattern *regexp.Regexp) string {
	lines := strings.Split(markdown, "\n")

	var content []string

	inSection := false

	// Pattern to match HTML comments like <!-- TRANSLATE_TEXT --> or <!-- TRANSLATE_ROWS: ... -->
	commentPattern := regexp.MustCompile(`^\s*<!--.*-->\s*$`)

	for _, line := range lines {
		if startPattern.MatchString(line) {
			inSection = true

			continue
		}

		if endPattern.MatchString(line) {
			break
		}

		if inSection {
			trimmed := strings.TrimSpace(line)
			// Skip empty lines and HTML comment tags
			if trimmed != "" && !commentPattern.MatchString(trimmed) {
				content = append(content, trimmed)
			}
		}
	}

	return strings.Join(content, " ")
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

// parseNotes extracts notes from the NOTES section.
func (p *Parser) parseNotes(markdown string) []string {
	var notes []string

	lines := strings.Split(markdown, "\n")
	inSection := false

	for _, line := range lines {
		if p.notesStartPattern.MatchString(line) {
			inSection = true

			continue
		}

		if p.notesEndPattern.MatchString(line) {
			break
		}

		if inSection {
			trimmed := strings.TrimSpace(line)
			if strings.HasPrefix(trimmed, "- ") {
				notes = append(notes, strings.TrimPrefix(trimmed, "- "))
			}
		}
	}

	return notes
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
	var photos []models.Photo

	if len(cells) > 8 {
		photosStr := strings.TrimSpace(cells[8])
		photos = parsePhotos(photosStr)
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
	casualties := p.parseCasualties(casualtiesStr)

	// Parse sources
	sources := p.parseSources(sourcesStr)

	// Create event ID
	eventID := fmt.Sprintf("%s-%s", currentDate, normalizeTime(timeStr))

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

// parseCasualties extracts casualty numbers from text.
func (p *Parser) parseCasualties(text string) models.CasualtyData {
	data := models.CasualtyData{
		Status: text,
		Raw:    text,
	}

	// Handle STATUS_NONE
	if strings.TrimSpace(text) == StatusNone {
		return data
	}

	// Handle status code format: DEAD:13,INJURED:7,MISSING:200
	if strings.Contains(text, ":") {
		// Split by comma for multiple status codes
		parts := strings.Split(text, ",")
		for _, part := range parts {
			part = strings.TrimSpace(part)

			// Handle DEAD:X or DEAD:X(ON_SITE:Y,TRANSIT:Z)
			if strings.HasPrefix(part, "DEAD:") {
				if num, ok := extractStatusCodeNumber(part, "DEAD"); ok {
					data.Deaths = num
				}
			}
			// Handle INJURED:X
			if strings.HasPrefix(part, "INJURED:") {
				if num, ok := extractStatusCodeNumber(part, "INJURED"); ok {
					data.Injured = num
				}
			}
			// Handle MISSING:X
			if strings.HasPrefix(part, "MISSING:") {
				if num, ok := extractStatusCodeNumber(part, "MISSING"); ok {
					data.Missing = num
				}
			}
			// Handle FIREFIGHTER_DEAD:X (add to deaths)
			if strings.HasPrefix(part, "FIREFIGHTER_DEAD:") {
				if num, ok := extractStatusCodeNumber(part, "FIREFIGHTER_DEAD"); ok {
					data.Deaths += num
				}
			}
			// Handle FIREFIGHTER_INJURED:X (add to injured)
			if strings.HasPrefix(part, "FIREFIGHTER_INJURED:") {
				if num, ok := extractStatusCodeNumber(part, "FIREFIGHTER_INJURED"); ok {
					data.Injured += num
				}
			}
		}

		return data
	}

	// Fallback: Look for patterns like "128死79傷" or "128死83傷，150名失蹤"
	parts := strings.Split(text, "，")

	for _, part := range parts {
		part = strings.TrimSpace(part)

		if strings.Contains(part, "死") {
			if num, ok := extractNumber(part, "死"); ok {
				data.Deaths = num
			}
		}

		if strings.Contains(part, "傷") {
			if num, ok := extractNumber(part, "傷"); ok {
				data.Injured = num
			}
		}

		if strings.Contains(part, "失蹤") || strings.Contains(part, "下落不明") {
			if num, ok := extractNumber(part, "失蹤|下落不明"); ok {
				data.Missing = num
			}
		}
	}

	return data
}

// parseSources extracts sources and URLs from text.
func (p *Parser) parseSources(text string) []models.EventSource {
	var sources []models.EventSource

	if text == "" || text == StatusNone {
		return sources
	}

	// First, try to match markdown links [text](url)
	matches := p.linkPattern.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		for _, match := range matches {
			if len(match) >= 3 {
				sources = append(sources, models.EventSource{
					Name: match[1],
					URL:  match[2],
				})
			}
		}

		return sources
	}

	// If no markdown links found, parse plain text sources separated by commas
	// e.g., "HK01", "SBS Australia", "HK01, SBS Australia"
	sourceNames := strings.Split(text, ",")
	for _, name := range sourceNames {
		name = strings.TrimSpace(name)
		if name != "" && name != StatusNone {
			sources = append(sources, models.EventSource{
				Name: name,
				URL:  "", // No URL available for plain text sources
			})
		}
	}

	return sources
}

// parsePhotos extracts photos and URLs from text.
func parsePhotos(text string) []models.Photo {
	var photos []models.Photo

	if text == "" {
		return photos
	}

	// Pattern: [text](url) - extract URL and caption
	re := regexp.MustCompile(`\[(.*?)\]\((.*?)\)`)

	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		for _, match := range matches {
			if len(match) >= 3 {
				photos = append(photos, models.Photo{
					Caption: match[1],
					URL:     match[2],
				})
			}
		}

		return photos
	}

	// If contains comma, split by comma and treat as Generic URLs
	if strings.Contains(text, ",") {
		urls := strings.Split(text, ",")
		for _, u := range urls {
			u = strings.TrimSpace(u)
			if u != "" {
				photos = append(photos, models.Photo{
					URL: u,
				})
			}
		}

		return photos
	}

	// Single URL case
	if text != "" {
		photos = append(photos, models.Photo{
			URL: text,
		})
	}

	return photos
}

// Helper functions

func padZero(s string) string {
	if len(s) == 1 {
		return "0" + s
	}

	return s
}

func normalizeTime(timeStr string) string {
	// Convert "14:50左右" or "14:50" to "14:50"
	timeStr = strings.TrimSpace(timeStr)
	timeStr = strings.ReplaceAll(timeStr, "左右", "")
	timeStr = strings.ReplaceAll(timeStr, "左轉", "")

	// Extract HH:MM format
	parts := strings.Split(timeStr, ":")
	if len(parts) >= 2 {
		return timeStr
	}

	return timeStr
}

// parseVideoURL extracts URL from markdown link format [text](url).
func parseVideoURL(videoStr string) string {
	if videoStr == "" {
		return ""
	}

	// Pattern: [text](url) - extract the URL part
	re := regexp.MustCompile(`\[.*?\]\((.*?)\)`)

	matches := re.FindStringSubmatch(videoStr)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}

	// If not in markdown format, check if it's a plain URL
	if strings.HasPrefix(videoStr, "http") {
		return videoStr
	}

	return ""
}

func isValidTime(timeStr string) bool {
	// Check special time IDs
	if timeStr == TimeAllDay || timeStr == TimeOngoing {
		return true
	}
	// Check if it matches HH:MM pattern
	matched, _ := regexp.MatchString(`\d{1,2}:\d{2}`, timeStr)

	return matched
}

func extractNumber(text, pattern string) (int, bool) {
	re := regexp.MustCompile(`(\d+)\s*` + pattern)

	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		var num int

		_, _ = fmt.Sscanf(matches[1], "%d", &num)

		return num, true
	}

	return 0, false
}

// extractStatusCodeNumber extracts number from status code format like "DEAD:13" or "DEAD:13(ON_SITE:9,TRANSIT:4)".
func extractStatusCodeNumber(text, prefix string) (int, bool) {
	// Pattern: PREFIX:NUMBER or PREFIX:NUMBER(...)
	re := regexp.MustCompile(prefix + `:(\d+)`)

	matches := re.FindStringSubmatch(text)
	if len(matches) > 1 {
		var num int

		_, _ = fmt.Sscanf(matches[1], "%d", &num)

		return num, true
	}

	return 0, false
}

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

			// Construct DateTime
			var dateTime string
			if timeStr == "TIME_ALL_DAY" || timeStr == "TIME_ONGOING" {
				dateTime = fmt.Sprintf("%sT00:00:00", dateStr)
			} else {
				dateTime = fmt.Sprintf("%sT%s:00", dateStr, normalizeTime(timeStr))
			}

			event := models.DetailedTimelineEvent{
				ID:            fmt.Sprintf("%s-%s-%d", dateStr, normalizeTime(timeStr), eventCount),
				Date:          dateStr,
				Time:          timeStr,
				DateTime:      dateTime,
				Event:         eventDesc,
				Category:      category,
				StatusNote:    statusNote,
				Sources:       p.parseSources(sourcesStr),
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

			event := models.LongTermTrackingEvent{
				ID:       fmt.Sprintf("ltt-%s-%d", dateStr, eventCount),
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

// ParseFileType detects the file type from the markdown content.
func (p *Parser) ParseFileType(content string) string {
	re := regexp.MustCompile(`<!--\s*FILE_TYPE:\s*(\w+)\s*-->`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}
