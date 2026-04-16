package output

import (
	"fmt"
	"io"
	"sort"
	"strings"
	"unicode/utf8"
)

const tableMaxColWidth = 40

// TableFormatter renders an array of objects as a Unicode box-drawing table.
// Non-array or non-object body values are formatted as plain JSON.
type TableFormatter struct {
	// Columns is the ordered list of column names to display. When empty,
	// all keys found in the first row are used.
	Columns []string
	// SortBy is an optional column name to sort rows by (ascending, string compare).
	SortBy string
}

func (f *TableFormatter) Format(w io.Writer, resp *Response, color bool) error {
	rows, ok := toRows(resp.Body)
	if !ok {
		// Fall back to JSON output for non-array bodies.
		return (&JSONFormatter{}).Format(w, resp, color)
	}
	if len(rows) == 0 {
		fmt.Fprintln(w, "(empty)")
		return nil
	}

	cols := f.Columns
	if len(cols) == 0 {
		cols = extractColumns(rows)
	}

	// Sort rows if requested.
	if f.SortBy != "" {
		sb := f.SortBy
		sort.SliceStable(rows, func(i, j int) bool {
			vi := cellString(rows[i][sb])
			vj := cellString(rows[j][sb])
			return vi < vj
		})
	}

	// Compute column widths (min = header width, max = tableMaxColWidth).
	widths := make([]int, len(cols))
	for i, c := range cols {
		widths[i] = utf8.RuneCountInString(c)
	}
	for _, row := range rows {
		for i, c := range cols {
			w2 := utf8.RuneCountInString(truncate(cellString(row[c]), tableMaxColWidth))
			if w2 > widths[i] {
				widths[i] = w2
			}
		}
	}

	writeSep := func(left, mid, right, horiz string) {
		var row strings.Builder
		row.WriteString(left)
		for i, width := range widths {
			row.WriteString(strings.Repeat(horiz, width+2))
			if i < len(widths)-1 {
				row.WriteString(mid)
			}
		}
		row.WriteString(right)
		row.WriteByte('\n')
		_, _ = io.WriteString(w, row.String())
	}

	writeRow := func(cells []string) {
		var row strings.Builder
		row.WriteString("│")
		for i, cell := range cells {
			padded := cell + strings.Repeat(" ", widths[i]-utf8.RuneCountInString(cell))
			row.WriteString(" ")
			row.WriteString(padded)
			row.WriteString(" │")
		}
		row.WriteByte('\n')
		_, _ = io.WriteString(w, row.String())
	}

	// Top border.
	writeSep("┌", "┬", "┐", "─")

	// Header row.
	writeRow(cols)

	// Header / body separator.
	writeSep("├", "┼", "┤", "─")

	// Data rows.
	for _, row := range rows {
		cells := make([]string, len(cols))
		for i, c := range cols {
			cells[i] = truncate(cellString(row[c]), tableMaxColWidth)
		}
		writeRow(cells)
	}

	// Bottom border.
	writeSep("└", "┴", "┘", "─")
	return nil
}

// toRows converts body to a slice of map rows. Returns false when body is not
// a []any of map[string]any.
func toRows(body any) ([]map[string]any, bool) {
	arr, ok := body.([]any)
	if !ok {
		return nil, false
	}
	rows := make([]map[string]any, 0, len(arr))
	for _, item := range arr {
		m, ok := item.(map[string]any)
		if !ok {
			return nil, false
		}
		rows = append(rows, m)
	}
	return rows, true
}

// extractColumns returns all keys seen in rows, in sorted order.
func extractColumns(rows []map[string]any) []string {
	seen := map[string]bool{}
	for _, row := range rows {
		for k := range row {
			seen[k] = true
		}
	}
	cols := make([]string, 0, len(seen))
	for k := range seen {
		cols = append(cols, k)
	}
	sort.Strings(cols)
	return cols
}

// cellString converts a cell value to a display string.
func cellString(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

// truncate shortens s to at most maxRunes runes, appending "…" if cut.
func truncate(s string, maxRunes int) string {
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	runes := []rune(s)
	return string(runes[:maxRunes-1]) + "…"
}
