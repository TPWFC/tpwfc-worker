package formatter

import (
	"strings"
	"testing"
)

func TestFormatMarkdown(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name: "Basic table formatting",
			input: `
| Header 1 | Header 2 |
| --- | --- |
| val 1 | val 2 |
`,
			expected: `
| Header 1 | Header 2 |
| -------- | -------- |
| val 1    | val 2    |
`,
		},
		{
			name: "Fix excessive dashes",
			input: `
| Col A | Col B |
| ---------------------- | ---------------------------------- |
| A | B |
`,
			expected: `
| Col A | Col B |
| ----- | ----- |
| A     | B     |
`,
		},
		{
			name: "Trim spaces in cells",
			input: `
|   Col A   |   Col B   |
| --- | --- |
|   val A   |   val B   |
`,
			expected: `
| Col A | Col B |
| ----- | ----- |
| val A | val B |
`,
		},
		{
			name: "Mixed content",
			input: `
# Title

| H1 | H2 |
| -- | -- |
| v1 | v2 |

Text after table.
`,
			expected: `
# Title

| H1  | H2  |
| --- | --- |
| v1  | v2  |

Text after table.
`,
		},
		{
			name: "Mixed CJK and ASCII",
			input: `
| Date | Event |
| --- | --- |
| 2025-01-01 | 消防處：增至83死。 |
| 2025-01-02 | Short text |
`,
			// "消防處：增至83死。" -> 9 Chinese chars (18 width) + 3 digits (3 width) + period (1 width) = 22 width ??
			// Let's verify width:
			// "消防處" (6) + "：" (2) + "增至" (4) + "83" (2) + "死" (2) + "。" (2) = 18?
			// Wait:
			// 消(2) 防(2) 處(2) ：(2) 增(2) 至(2) 8(1) 3(1) 死(2) 。(2)
			// Total: 18 width.
			// "Short text" -> 10 width.
			// Max width is 18.
			// Date col: 10 width.
			expected: `
| Date       | Event              |
| ---------- | ------------------ |
| 2025-01-01 | 消防處：增至83死。 |
| 2025-01-02 | Short text         |
`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FormatMarkdown(strings.TrimSpace(tt.input))
			if err != nil {
				t.Errorf("FormatMarkdown() error = %v", err)

				return
			}

			if strings.TrimSpace(got) != strings.TrimSpace(tt.expected) {
				t.Errorf("FormatMarkdown() = \n%v\nwant \n%v", got, tt.expected)
			}
		})
	}
}
