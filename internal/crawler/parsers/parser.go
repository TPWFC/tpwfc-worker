// Package parsers provides web scraping and markdown parsing functionality for extracting fire timeline data.
package parsers

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"strings"
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

// Column Constants for dynamic parsing.
const (
	ColDate       = "DATE"
	ColTime       = "TIME"
	ColEvent      = "EVENT"
	ColCategory   = "CATEGORY"
	ColCasualties = "CASUALTIES"
	ColSource     = "SOURCE"
	ColVideo      = "VIDEO"
	ColPhoto      = "PHOTO"
	ColEnd        = "END"
)

// NormalizeHeader standardizes header names to internal constants.
func NormalizeHeader(header string) string {
	h := strings.ToUpper(strings.TrimSpace(header))
	switch h {
	case "DATE", "日期":
		return ColDate
	case "TIME", "時間", "时间":
		return ColTime
	case "EVENT", "事件", "DESCRIPTION", "描述":
		return ColEvent
	case "CATEGORY", "類別", "类别":
		return ColCategory
	case "CASUALTIES", "死傷狀況", "死伤状况":
		return ColCasualties
	case "SOURCE", "SOURCES", "來源", "来源":
		return ColSource
	case "VIDEO", "影片", "视频":
		return ColVideo
	case "PHOTO", "PHOTOS", "圖片", "图片", "PHOTO/IMAGE":
		return ColPhoto
	case "END", "結束", "结束":
		return ColEnd
	default:
		return h
	}
}

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

// generateEventID creates a unique event ID using SHA-256 hash.
// Hash combines only locale-independent fields: date, time, and category.
// Excludes description, source names, source URLs, video URLs, and photo URLs
// as these can differ between locale files for the same logical event.
func generateEventID(date, time, category string) string {
	// Combine only locale-independent fields with a delimiter
	data := strings.Join([]string{
		date,
		time,
		category,
	}, "|")

	// Generate SHA-256 hash
	hash := sha256.Sum256([]byte(data))
	hashStr := hex.EncodeToString(hash[:])

	// Return first 12 characters of the hash
	return hashStr[:12]
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

// parseCasualties extracts casualty numbers from text.
func (p *Parser) parseCasualties(text string) CasualtyData {
	data := CasualtyData{
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
					data.Items = append(data.Items, CasualtyItem{Type: "DEAD", Count: num})
				}
			}
			// Handle INJURED:X
			if strings.HasPrefix(part, "INJURED:") {
				if num, ok := extractStatusCodeNumber(part, "INJURED"); ok {
					data.Items = append(data.Items, CasualtyItem{Type: "INJURED", Count: num})
				}
			}
			// Handle MISSING:X
			if strings.HasPrefix(part, "MISSING:") {
				if num, ok := extractStatusCodeNumber(part, "MISSING"); ok {
					data.Items = append(data.Items, CasualtyItem{Type: "MISSING", Count: num})
				}
			}
			// Handle FIREFIGHTER_DEAD:X
			if strings.HasPrefix(part, "FIREFIGHTER_DEAD:") {
				if num, ok := extractStatusCodeNumber(part, "FIREFIGHTER_DEAD"); ok {
					data.Items = append(data.Items, CasualtyItem{Type: "FIREFIGHTER_DEAD", Count: num})
				}
			}
			// Handle FIREFIGHTER_INJURED:X
			if strings.HasPrefix(part, "FIREFIGHTER_INJURED:") {
				if num, ok := extractStatusCodeNumber(part, "FIREFIGHTER_INJURED"); ok {
					data.Items = append(data.Items, CasualtyItem{Type: "FIREFIGHTER_INJURED", Count: num})
				}
			}
			// Handle REMAINING_CASES:X
			if strings.HasPrefix(part, "REMAINING_CASES:") {
				if num, ok := extractStatusCodeNumber(part, "REMAINING_CASES"); ok {
					data.Items = append(data.Items, CasualtyItem{Type: "REMAINING_CASES", Count: num})
				}
			}
			// Handle UNIDENTIFIED:X
			if strings.HasPrefix(part, "UNIDENTIFIED:") {
				if num, ok := extractStatusCodeNumber(part, "UNIDENTIFIED"); ok {
					data.Items = append(data.Items, CasualtyItem{Type: "UNIDENTIFIED", Count: num})
				}
			}
		}

		if len(data.Items) > 0 {
			data.Status = "STATUS_UPDATE"
		}

		return data
	}

	// Fallback: Look for patterns like "128死79傷" or "128死83傷，150名失蹤"
	parts := strings.Split(text, "，")

	for _, part := range parts {
		part = strings.TrimSpace(part)

		if strings.Contains(part, "死") {
			if num, ok := extractNumber(part, "死"); ok {
				data.Items = append(data.Items, CasualtyItem{Type: "DEAD", Count: num})
			}
		}

		if strings.Contains(part, "傷") {
			if num, ok := extractNumber(part, "傷"); ok {
				data.Items = append(data.Items, CasualtyItem{Type: "INJURED", Count: num})
			}
		}

		if strings.Contains(part, "失蹤") || strings.Contains(part, "下落不明") {
			if num, ok := extractNumber(part, "失蹤|下落不明"); ok {
				data.Items = append(data.Items, CasualtyItem{Type: "MISSING", Count: num})
			}
		}
	}

	if len(data.Items) > 0 {
		data.Status = "STATUS_UPDATE"
	}

	return data
}

// CasualtyItem is a temporary type for internal parsing.
type CasualtyItem struct {
	Type  string
	Count int
}

// CasualtyData is a temporary type for internal parsing - maps to models.CasualtyData.
type CasualtyData struct {
	Status string
	Raw    string
	Items  []CasualtyItem
}

// parseSources extracts sources and URLs from text.
func (p *Parser) parseSources(text string) []EventSource {
	var sources []EventSource

	if text == "" || text == StatusNone {
		return sources
	}

	// First, try to match markdown links [text](url)
	matches := p.linkPattern.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		for _, match := range matches {
			if len(match) >= 3 {
				sources = append(sources, EventSource{
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
			sources = append(sources, EventSource{
				Name: name,
				URL:  "", // No URL available for plain text sources
			})
		}
	}

	return sources
}

// EventSource is a temporary type for internal parsing - maps to models.EventSource.
type EventSource struct {
	Name string
	URL  string
}

// parsePhotos extracts photos and URLs from text.
func parsePhotos(text string) []Photo {
	var photos []Photo

	if text == "" {
		return photos
	}

	// Pattern: [text](url) - extract URL and caption
	re := regexp.MustCompile(`\[(.*?)\]\((.*?)\)`)

	matches := re.FindAllStringSubmatch(text, -1)
	if len(matches) > 0 {
		for _, match := range matches {
			if len(match) >= 3 {
				photos = append(photos, Photo{
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
				photos = append(photos, Photo{
					URL: u,
				})
			}
		}

		return photos
	}

	// Single URL case
	if text != "" {
		photos = append(photos, Photo{
			URL: text,
		})
	}

	return photos
}

// Photo is a temporary type for internal parsing - maps to models.Photo.
type Photo struct {
	Caption string
	URL     string
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

// ParseFileType detects the file type from the markdown content.
func (p *Parser) ParseFileType(content string) string {
	re := regexp.MustCompile(`<!--\s*FILE_TYPE:\s*(\w+)\s*-->`)
	matches := re.FindStringSubmatch(content)
	if len(matches) > 1 {
		return matches[1]
	}
	return ""
}
