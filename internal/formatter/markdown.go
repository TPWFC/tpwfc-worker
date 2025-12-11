// Package formatter provides markdown formatting utilities.
package formatter

import (
	"strings"

	"tpwfc/pkg/metadata"

	"github.com/mattn/go-runewidth"
)

// FormatMarkdown takes a raw markdown string and formats it,
// specifically focusing on fixing table formatting issues.
// It also strips any existing metadata block, returning only the cleaned formatted content.
func FormatMarkdown(content string) (string, error) {
	// Strip metadata before formatting
	_, cleanContent := metadata.Extract(content)

	lines := strings.Split(cleanContent, "\n")

	var formattedLines []string

	var tableBuffer []string

	for i := 0; i < len(lines); i++ {
		line := lines[i]
		trimmedLine := strings.TrimSpace(line)

		// Check if the line looks like a table row
		if strings.HasPrefix(trimmedLine, "|") && strings.HasSuffix(trimmedLine, "|") {
			tableBuffer = append(tableBuffer, line)

			continue
		}

		// If we were buffering a table and hit a non-table line, process the buffer
		if len(tableBuffer) > 0 {
			formattedLines = append(formattedLines, processTable(tableBuffer)...)
			tableBuffer = nil
		}

		formattedLines = append(formattedLines, line)
	}

	// Process any remaining table at the end of the file
	if len(tableBuffer) > 0 {
		formattedLines = append(formattedLines, processTable(tableBuffer)...)
	}

	return strings.Join(formattedLines, "\n"), nil
}

func processTable(rows []string) []string {
	// If it's just one line, it's not really a table we can format nicely (needs header+separator)
	if len(rows) == 0 {
		return nil
	}

	// 1. Parse all cells
	var table [][]string

	for _, row := range rows {
		// Remove leading/trailing pipes for splitting, but keep them in mind for reconstruction
		// Standard markdown table: | cell1 | cell2 |
		// Split by pipe
		parts := strings.Split(row, "|")

		// The split will result in empty strings at start/end if the line starts/ends with pipe
		// e.g. "| a | b |" -> ["", " a ", " b ", ""]
		if len(parts) > 0 && strings.TrimSpace(parts[0]) == "" {
			parts = parts[1:]
		}

		if len(parts) > 0 && strings.TrimSpace(parts[len(parts)-1]) == "" {
			parts = parts[:len(parts)-1]
		}

		var cells []string
		for _, p := range parts {
			cells = append(cells, strings.TrimSpace(p))
		}

		table = append(table, cells)
	}

	// 2. Validate table structure
	if len(table) == 0 {
		return rows // return original if parsing failed
	}

	colCount := len(table[0])

	// Identify separator row (usually 2nd row, index 1)
	separatorRowIdx := -1

	if len(table) > 1 {
		isSep := true

		for _, cell := range table[1] {
			trim := strings.TrimSpace(cell)
			trim = strings.ReplaceAll(trim, "-", "")
			trim = strings.ReplaceAll(trim, ":", "")

			if trim != "" {
				isSep = false

				break
			}
		}

		if isSep {
			separatorRowIdx = 1
		}
	}

	// 3. Calculate max widths (using display width)
	colWidths := make([]int, colCount)

	for rIdx, row := range table {
		// Skip separator row for width calculation
		if rIdx == separatorRowIdx {
			continue
		}

		for i := 0; i < len(row) && i < colCount; i++ {
			width := runewidth.StringWidth(row[i])
			if width > colWidths[i] {
				colWidths[i] = width
			}
		}
	}

	// Ensure min width for separator (usually 3 dashes "---")
	for i := range colWidths {
		if colWidths[i] < 3 {
			colWidths[i] = 3
		}
	}

	// 4. Reconstruct lines
	var result []string

	for i, row := range table {
		var sb strings.Builder

		sb.WriteString("|")

		isSeparator := (i == separatorRowIdx)

		for j := 0; j < colCount; j++ {
			sb.WriteString(" ")

			content := ""
			if j < len(row) {
				content = row[j]
			}

			if isSeparator {
				// Reconstruct separator based on alignment
				dashCount := colWidths[j]
				sb.WriteString(strings.Repeat("-", dashCount))
			} else {
				sb.WriteString(content)
				// Pad with spaces based on display width
				contentWidth := runewidth.StringWidth(content)

				padding := colWidths[j] - contentWidth
				if padding > 0 {
					sb.WriteString(strings.Repeat(" ", padding))
				}
			}

			sb.WriteString(" |")
		}

		result = append(result, sb.String())
	}

	return result
}
