package main

import (
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/rest-sh/restish/v2/plugin"
)

func main() {
	manifest := plugin.Manifest{
		Name:              "csv",
		Version:           "1.0.0",
		Description:       "Formatter plugin that renders object-shaped responses as CSV",
		RestishAPIVersion: 2,
		Hooks:             []string{"formatter"},
		FormatterNames:    []string{"csv"},
	}
	if plugin.HandleStartupFlags(os.Stdout, manifest, nil) {
		return
	}

	dec := plugin.NewDecoder(os.Stdin)
	var req plugin.FormatterRequest
	if err := dec.ReadMessage(&req); err != nil {
		fail(fmt.Errorf("read formatter request: %w", err))
	}
	if req.Type != "formatter" {
		fail(fmt.Errorf("expected formatter request, got %q", req.Type))
	}
	if req.Format != "csv" {
		fail(fmt.Errorf("unsupported formatter %q", req.Format))
	}
	if req.Event != "start" {
		fail(fmt.Errorf("expected first formatter event to be start, got %q", req.Event))
	}

	formatter := newCSVFormatter(os.Stdout)
	if err := formatter.Handle(req); err != nil {
		fail(err)
	}

	for {
		var req plugin.FormatterRequest
		if err := dec.ReadMessage(&req); err != nil {
			fail(fmt.Errorf("read formatter request: %w", err))
		}
		if req.Type != "formatter" {
			fail(fmt.Errorf("expected formatter request, got %q", req.Type))
		}
		if req.Format != "csv" {
			fail(fmt.Errorf("unsupported formatter %q", req.Format))
		}
		if err := formatter.Handle(req); err != nil {
			fail(err)
		}
		if req.Event == "end" {
			break
		}
	}
}

type csvFormatter struct {
	writer  *csv.Writer
	headers []string
	index   map[string]struct{}
}

func newCSVFormatter(w io.Writer) *csvFormatter {
	return &csvFormatter{writer: csv.NewWriter(w)}
}

// FormatCSV formats one value as CSV using the same logic the formatter
// session uses for a non-streaming response body.
func FormatCSV(w io.Writer, value any) error {
	f := newCSVFormatter(w)
	if err := f.Handle(plugin.FormatterRequest{
		Event: "start",
		Response: plugin.FormatterResponse{
			Body: value,
		},
	}); err != nil {
		return err
	}
	return f.Handle(plugin.FormatterRequest{Event: "end"})
}

func (f *csvFormatter) Handle(req plugin.FormatterRequest) error {
	switch req.Event {
	case "start", "item":
		return f.writeValue(req.Response.Body)
	case "end":
		f.writer.Flush()
		if err := f.writer.Error(); err != nil {
			return fmt.Errorf("flush csv: %w", err)
		}
		return nil
	default:
		return fmt.Errorf("unsupported formatter event %q", req.Event)
	}
}

func (f *csvFormatter) writeValue(value any) error {
	rows, err := csvRows(value)
	if err != nil {
		return err
	}
	if len(rows) == 0 {
		return nil
	}

	if len(f.headers) == 0 {
		f.headers = collectHeaders(rows)
		f.index = make(map[string]struct{}, len(f.headers))
		for _, header := range f.headers {
			f.index[header] = struct{}{}
		}
		if err := f.writer.Write(f.headers); err != nil {
			return fmt.Errorf("write header: %w", err)
		}
	}

	for i, row := range rows {
		if err := f.ensureStableColumns(row); err != nil {
			return fmt.Errorf("write row %d: %w", i, err)
		}
		record := make([]string, len(f.headers))
		for j, header := range f.headers {
			record[j] = csvCell(row[header])
		}
		if err := f.writer.Write(record); err != nil {
			return fmt.Errorf("write row %d: %w", i, err)
		}
	}
	f.writer.Flush()
	if err := f.writer.Error(); err != nil {
		return fmt.Errorf("flush csv: %w", err)
	}
	return nil
}

func (f *csvFormatter) ensureStableColumns(row map[string]any) error {
	var extra []string
	for key := range row {
		if _, ok := f.index[key]; !ok {
			extra = append(extra, key)
		}
	}
	if len(extra) == 0 {
		return nil
	}
	sort.Strings(extra)
	return fmt.Errorf("csv formatter requires stable columns after the header; unexpected fields: %s", strings.Join(extra, ", "))
}

func csvRows(value any) ([]map[string]any, error) {
	switch data := value.(type) {
	case nil:
		return nil, nil
	case map[string]any:
		return []map[string]any{data}, nil
	case []any:
		rows := make([]map[string]any, 0, len(data))
		for i, item := range data {
			row, ok := item.(map[string]any)
			if !ok {
				return nil, fmt.Errorf("csv formatter expects each array item to be an object (row %d)", i)
			}
			rows = append(rows, row)
		}
		return rows, nil
	default:
		return nil, fmt.Errorf("csv formatter expects an object or array of objects")
	}
}

func collectHeaders(rows []map[string]any) []string {
	seen := map[string]struct{}{}
	for _, row := range rows {
		for key := range row {
			seen[key] = struct{}{}
		}
	}

	headers := make([]string, 0, len(seen))
	for key := range seen {
		headers = append(headers, key)
	}
	sort.Strings(headers)
	return headers
}

func csvCell(v any) string {
	switch data := v.(type) {
	case nil:
		return ""
	case string:
		return data
	default:
		encoded, err := json.Marshal(v)
		if err != nil {
			return fmt.Sprintf("%v", v)
		}
		return string(encoded)
	}
}

func fail(err error) {
	fmt.Fprintln(os.Stderr, err)
	os.Exit(1)
}
