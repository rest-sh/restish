package output

import (
	"encoding/json"
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
	// SortBy is an optional column name to sort rows by ascending value.
	// Numeric cells sort numerically; other cells sort by display string.
	SortBy string
}

func (f *TableFormatter) Format(w io.Writer, resp *Response, color bool) error {
	if resp == nil {
		resp = &Response{}
	}
	rows, ok := toRows(resp.Body)
	if !ok {
		// Fall back to JSON output for non-array bodies.
		return (&JSONFormatter{}).Format(w, resp, color)
	}
	if len(rows) == 0 {
		_, err := fmt.Fprintln(w, "(empty)")
		return err
	}

	cols := f.Columns
	if len(cols) == 0 {
		cols = extractColumns(rows)
	}

	// Sort rows if requested.
	if f.SortBy != "" {
		sb := f.SortBy
		sort.SliceStable(rows, func(i, j int) bool {
			return compareTableCells(rows[i][sb], rows[j][sb]) < 0
		})
	}

	cells := make([][]string, len(rows))
	for r, row := range rows {
		cells[r] = make([]string, len(cols))
		for i, c := range cols {
			cells[r][i] = truncate(cellString(row[c]), tableMaxColWidth)
		}
	}

	// Compute column widths (min = header width, max = tableMaxColWidth).
	widths := make([]int, len(cols))
	for i, c := range cols {
		widths[i] = utf8.RuneCountInString(c)
	}
	for _, row := range cells {
		for i, cell := range row {
			w2 := utf8.RuneCountInString(cell)
			if w2 > widths[i] {
				widths[i] = w2
			}
		}
	}

	writeSep := func(left, mid, right, horiz string) error {
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
		_, err := io.WriteString(w, row.String())
		return err
	}

	writeRow := func(cells []string) error {
		var row strings.Builder
		row.WriteString("│")
		for i, cell := range cells {
			padded := cell + strings.Repeat(" ", widths[i]-utf8.RuneCountInString(cell))
			row.WriteString(" ")
			row.WriteString(padded)
			row.WriteString(" │")
		}
		row.WriteByte('\n')
		_, err := io.WriteString(w, row.String())
		return err
	}

	// Top border.
	if err := writeSep("┌", "┬", "┐", "─"); err != nil {
		return err
	}

	// Header row.
	if err := writeRow(cols); err != nil {
		return err
	}

	// Header / body separator.
	if err := writeSep("├", "┼", "┤", "─"); err != nil {
		return err
	}

	// Data rows.
	for _, row := range cells {
		if err := writeRow(row); err != nil {
			return err
		}
	}

	// Bottom border.
	return writeSep("└", "┴", "┘", "─")
}

// toRows converts body to a slice of map rows. Returns false when body is not
// a []any of map[string]any.
func toRows(body any) ([]map[string]any, bool) {
	if m, ok := body.(map[string]any); ok {
		return []map[string]any{m}, true
	}
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

// extractColumns returns all keys seen in rows, preserving first-row key
// order and appending any additional keys from later rows in sorted order.
func extractColumns(rows []map[string]any) []string {
	if len(rows) == 0 {
		return nil
	}
	seen := map[string]bool{}
	var cols []string

	// Preserve first-row key order (Go map iteration is random, so we use the
	// fact that any additional key from a later row is truly "extra").
	for k := range rows[0] {
		if !seen[k] {
			seen[k] = true
			cols = append(cols, k)
		}
	}
	sort.Strings(cols) // stabilise first-row order alphabetically

	// Collect extra keys from subsequent rows.
	var extra []string
	for _, row := range rows[1:] {
		for k := range row {
			if !seen[k] {
				seen[k] = true
				extra = append(extra, k)
			}
		}
	}
	sort.Strings(extra)
	return append(cols, extra...)
}

// cellString converts a cell value to a display string.
func cellString(v any) string {
	if v == nil {
		return ""
	}
	return fmt.Sprintf("%v", v)
}

func compareTableCells(a, b any) int {
	if af, ok := tableNumber(a); ok {
		if bf, ok := tableNumber(b); ok {
			switch {
			case af < bf:
				return -1
			case af > bf:
				return 1
			default:
				return 0
			}
		}
	}
	return strings.Compare(cellString(a), cellString(b))
}

func tableNumber(v any) (float64, bool) {
	switch n := v.(type) {
	case int:
		return float64(n), true
	case int8:
		return float64(n), true
	case int16:
		return float64(n), true
	case int32:
		return float64(n), true
	case int64:
		return float64(n), true
	case uint:
		return float64(n), true
	case uint8:
		return float64(n), true
	case uint16:
		return float64(n), true
	case uint32:
		return float64(n), true
	case uint64:
		return float64(n), true
	case float32:
		return float64(n), true
	case float64:
		return n, true
	case json.Number:
		f, err := n.Float64()
		return f, err == nil
	default:
		return 0, false
	}
}

// truncate shortens s to at most maxRunes runes, appending "…" if cut.
func truncate(s string, maxRunes int) string {
	if maxRunes <= 0 {
		return ""
	}
	if utf8.RuneCountInString(s) <= maxRunes {
		return s
	}
	count := 0
	for i := range s {
		if count == maxRunes-1 {
			return s[:i] + "…"
		}
		count++
	}
	return s
}
